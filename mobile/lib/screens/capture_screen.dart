import 'dart:async';
import 'dart:io';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:image_picker/image_picker.dart';
import 'package:latlong2/latlong.dart';
import 'package:schmutzfink_mobile/api/client.dart';
import 'package:schmutzfink_mobile/api/models.dart';
import 'package:schmutzfink_mobile/exif_gps.dart';
import 'package:schmutzfink_mobile/screens/map_pin_screen.dart';
import 'package:schmutzfink_mobile/settings.dart';
import 'package:schmutzfink_mobile/upload_history.dart';
import 'package:schmutzfink_mobile/upload_queue.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:url_launcher/url_launcher.dart';

enum _GpsKind { device, exif, map, none }

/// Ab dieser Horizontalgenauigkeit (Meter) gilt der Fix als ungenau.
const _gpsWarnMeters = 30.0;

class CaptureScreen extends StatefulWidget {
  const CaptureScreen({
    super.key,
    required this.client,
    required this.settings,
    required this.onLogout,
  });

  final ApiClient client;
  final AppSettings settings;
  final Future<void> Function() onLogout;

  @override
  State<CaptureScreen> createState() => _CaptureScreenState();
}

class _CaptureScreenState extends State<CaptureScreen> with WidgetsBindingObserver {
  final _noteCtrl = TextEditingController();
  final _tagsCtrl = TextEditingController();
  final _picker = ImagePicker();

  File? _photo;
  double? _lat;
  double? _lon;
  double? _gpsAccuracyM;
  DateTime? _capturedAt;
  _GpsKind _gpsKind = _GpsKind.none;
  bool _enqueuing = false;
  bool _loadingRecent = false;
  bool _seriesMode = false;
  String? _caseId;
  String? _status;
  String? _error;
  List<RecentUpload> _recent = [];
  List<CaseSummary> _cases = [];
  UploadHistory? _history;
  UploadQueue? _queue;
  StreamSubscription<List<ConnectivityResult>>? _connectivitySub;
  bool _hadOffline = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _bootstrap();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _connectivitySub?.cancel();
    _noteCtrl.dispose();
    _tagsCtrl.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _queue?.drain();
    }
  }

  Future<void> _bootstrap() async {
    final prefs = await SharedPreferences.getInstance();
    _history = UploadHistory(prefs);
    _queue = UploadQueue(
      prefs,
      send: (item) => widget.client.uploadPhoto(
        file: File(item.filePath),
        lat: item.lat,
        lon: item.lon,
        note: item.note,
        tags: item.tags,
        caseId: item.caseId,
        capturedAt: item.capturedAt,
      ),
    );
    _queue!.onChanged = () {
      if (mounted) setState(() {});
    };
    _queue!.onUploaded = (entry) async {
      await _history?.prepend(entry);
      if (!mounted) return;
      setState(() {
        _recent = [entry, ..._recent.where((e) => e.id != entry.id)];
        _status = entry.duplicate
            ? 'Datei war schon vorhanden — ${entry.locationLabel}.'
            : 'Gespeichert — ${entry.locationLabel}.';
        _error = null;
      });
      await _refreshRecent();
    };
    _queue!.onAuthLost = (_) async {
      await widget.onLogout();
    };

    _connectivitySub = Connectivity().onConnectivityChanged.listen((results) {
      final online = results.any((r) => r != ConnectivityResult.none);
      if (!online) {
        _hadOffline = true;
        return;
      }
      if (_hadOffline) {
        _hadOffline = false;
        _queue?.drain();
      } else if ((_queue?.pendingCount ?? 0) > 0) {
        _queue?.drain();
      }
    });

    final local = _history!.load();
    await _queue!.load();
    if (mounted) {
      setState(() {
        if (local.isNotEmpty) _recent = local;
      });
    }
    await Future.wait([_refreshRecent(), _loadCases(), _queue!.drain()]);
  }

  Future<void> _loadCases() async {
    try {
      final items = await widget.client.listCases();
      if (!mounted) return;
      setState(() {
        _cases = items.where((c) => c.isOpen).toList();
        if (_caseId != null && !_cases.any((c) => c.id == _caseId)) {
          _caseId = null;
        }
      });
    } on ApiException catch (e) {
      if (e.statusCode == 401) {
        await widget.onLogout();
      }
    } catch (_) {
      // Offline: Vorgänge bleiben leer / bisherige Auswahl.
    }
  }

  Future<void> _refreshRecent() async {
    setState(() => _loadingRecent = true);
    try {
      final server = await widget.client.listRecent(limit: 15);
      if (!mounted) return;
      setState(() => _recent = server);
    } on ApiException catch (e) {
      if (e.statusCode == 401) {
        await widget.onLogout();
        return;
      }
    } catch (_) {
      // Offline: lokalen Verlauf behalten.
    } finally {
      if (mounted) setState(() => _loadingRecent = false);
    }
  }

  Future<Position?> _readDeviceGps() async {
    final enabled = await Geolocator.isLocationServiceEnabled();
    if (!enabled) {
      throw ApiException('Standortdienste sind ausgeschaltet.');
    }
    var permission = await Geolocator.checkPermission();
    if (permission == LocationPermission.denied) {
      permission = await Geolocator.requestPermission();
    }
    if (permission == LocationPermission.denied) {
      throw ApiException('Standortberechtigung verweigert.');
    }
    if (permission == LocationPermission.deniedForever) {
      throw ApiException(
        'Standortberechtigung dauerhaft verweigert — bitte in den Systemeinstellungen erlauben.',
      );
    }
    return Geolocator.getCurrentPosition(
      locationSettings: const LocationSettings(
        accuracy: LocationAccuracy.high,
        timeLimit: Duration(seconds: 20),
      ),
    );
  }

  void _clearSelectionMessages() {
    _error = null;
    _status = null;
  }

  void _clearPhotoOnly() {
    _photo = null;
    _lat = null;
    _lon = null;
    _gpsAccuracyM = null;
    _gpsKind = _GpsKind.none;
    _capturedAt = null;
  }

  String _gpsStatusText() {
    final acc = _gpsAccuracyM;
    final accPart = acc != null && acc.isFinite ? ' (±${acc.round()} m)' : '';
    switch (_gpsKind) {
      case _GpsKind.device:
        return 'Foto bereit — GPS vom Gerät ${_lat!.toStringAsFixed(5)}, ${_lon!.toStringAsFixed(5)}$accPart';
      case _GpsKind.exif:
        return 'Foto bereit — GPS aus EXIF ${_lat!.toStringAsFixed(5)}, ${_lon!.toStringAsFixed(5)}';
      case _GpsKind.map:
        return 'Foto bereit — Ort von der Karte ${_lat!.toStringAsFixed(5)}, ${_lon!.toStringAsFixed(5)}';
      case _GpsKind.none:
        return 'Foto bereit — ohne GPS (Ort auf Karte setzen oder später in der Web-UI).';
    }
  }

  String? _gpsQualityWarning() {
    final acc = _gpsAccuracyM;
    if (_gpsKind != _GpsKind.device || acc == null || !acc.isFinite) return null;
    if (acc > _gpsWarnMeters) {
      return 'GPS ungenau (±${acc.round()} m) — besser Ort auf der Karte prüfen.';
    }
    return null;
  }

  Future<void> _applyDeviceGpsOrWarn() async {
    try {
      final pos = await _readDeviceGps();
      if (pos == null) return;
      setState(() {
        _lat = pos.latitude;
        _lon = pos.longitude;
        _gpsAccuracyM = pos.accuracy;
        _gpsKind = _GpsKind.device;
        _status = _gpsStatusText();
        _error = _gpsQualityWarning();
      });
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _status = _gpsStatusText();
      });
    } catch (_) {
      setState(() {
        _error = 'GPS konnte nicht gelesen werden — Ort auf der Karte setzen.';
        _status = _gpsStatusText();
      });
    }
  }

  Future<void> _takePhoto() async {
    setState(_clearSelectionMessages);
    try {
      final shot = await _picker.pickImage(
        source: ImageSource.camera,
        imageQuality: 92,
        preferredCameraDevice: CameraDevice.rear,
        requestFullMetadata: true,
      );
      if (shot == null) return;

      double? lat;
      double? lon;
      double? accuracy;
      var kind = _GpsKind.none;
      String? gpsWarn;
      try {
        final pos = await _readDeviceGps();
        if (pos != null) {
          lat = pos.latitude;
          lon = pos.longitude;
          accuracy = pos.accuracy;
          kind = _GpsKind.device;
          if (accuracy.isFinite && accuracy > _gpsWarnMeters) {
            gpsWarn = 'GPS ungenau (±${accuracy.round()} m) — besser Ort auf der Karte prüfen.';
          }
        }
      } on ApiException catch (e) {
        gpsWarn = '${e.message} — Ort auf der Karte setzen.';
      } catch (_) {
        gpsWarn = 'GPS konnte nicht gelesen werden — Ort auf der Karte setzen.';
      }

      setState(() {
        _photo = File(shot.path);
        _lat = lat;
        _lon = lon;
        _gpsAccuracyM = accuracy;
        _gpsKind = kind;
        _capturedAt = DateTime.now();
        _error = gpsWarn;
        _status = _gpsStatusText();
      });

      if (kind == _GpsKind.none && mounted) {
        final wantMap = await showDialog<bool>(
          context: context,
          builder: (ctx) => AlertDialog(
            title: const Text('Kein GPS'),
            content: const Text('Ort jetzt auf der Karte setzen?'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Später')),
              FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Karte')),
            ],
          ),
        );
        if (wantMap == true) await _pickMapPin();
      }
    } catch (_) {
      setState(() => _error = 'Kamera nicht verfügbar.');
    }
  }

  Future<void> _pickGallery({bool multi = false}) async {
    setState(_clearSelectionMessages);
    try {
      if (multi) {
        final shots = await _picker.pickMultiImage(
          imageQuality: 92,
          requestFullMetadata: true,
        );
        if (shots.isEmpty) return;
        await _enqueueMany(shots.map((s) => File(s.path)).toList());
        return;
      }

      final shot = await _picker.pickImage(
        source: ImageSource.gallery,
        imageQuality: 92,
        requestFullMetadata: true,
      );
      if (shot == null) return;

      final file = File(shot.path);
      final meta = await readExifMeta(file);
      setState(() {
        _photo = file;
        _lat = meta.lat;
        _lon = meta.lon;
        _gpsAccuracyM = null;
        _gpsKind = meta.hasGps ? _GpsKind.exif : _GpsKind.none;
        _capturedAt = meta.capturedAt ?? DateTime.now();
        _error = meta.hasGps
            ? null
            : 'Kein EXIF-GPS — Ort auf der Karte setzen oder später in der Web-UI.';
        _status = _gpsStatusText();
      });
    } catch (_) {
      setState(() => _error = 'Galerie nicht verfügbar.');
    }
  }

  Future<void> _pickMapPin() async {
    final result = await Navigator.of(context).push<LatLng>(
      MaterialPageRoute(
        builder: (_) => MapPinScreen(initialLat: _lat, initialLon: _lon),
      ),
    );
    if (result == null || !mounted) return;
    setState(() {
      _lat = result.latitude;
      _lon = result.longitude;
      _gpsAccuracyM = null;
      _gpsKind = _GpsKind.map;
      _error = null;
      _status = _gpsStatusText();
    });
  }

  Future<void> _enqueueUpload({bool continueSeries = false}) async {
    final photo = _photo;
    final queue = _queue;
    if (photo == null) {
      setState(() => _error = 'Bitte zuerst ein Foto aufnehmen oder auswählen.');
      return;
    }
    if (queue == null) {
      setState(() => _error = 'Warteschlange noch nicht bereit.');
      return;
    }
    setState(() {
      _enqueuing = true;
      _error = null;
      _status = 'Wird in die Warteschlange gelegt…';
    });
    try {
      final item = await queue.enqueue(
        file: photo,
        lat: _lat,
        lon: _lon,
        note: _noteCtrl.text,
        tags: _tagsCtrl.text,
        caseId: _caseId ?? '',
        capturedAt: _capturedAt,
      );
      if (!mounted) return;
      setState(() {
        _status = 'In Warteschlange — ${item.locationLabel}.';
        _clearPhotoOnly();
      });
      // Upload nur im Hintergrund — Kamera nicht auf Netz warten lassen.
      unawaited(queue.drain());
      if (continueSeries || _seriesMode) {
        await _takePhoto();
      }
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Konnte nicht in die Warteschlange legen.';
        _status = null;
      });
    } finally {
      if (mounted) setState(() => _enqueuing = false);
    }
  }

  Future<void> _enqueueMany(List<File> files) async {
    final queue = _queue;
    if (queue == null) {
      setState(() => _error = 'Warteschlange noch nicht bereit.');
      return;
    }
    setState(() {
      _enqueuing = true;
      _error = null;
      _status = '${files.length} Fotos werden eingereiht…';
    });
    try {
      var n = 0;
      for (final file in files) {
        final meta = await readExifMeta(file);
        await queue.enqueue(
          file: file,
          lat: meta.lat,
          lon: meta.lon,
          note: _noteCtrl.text,
          tags: _tagsCtrl.text,
          caseId: _caseId ?? '',
          capturedAt: meta.capturedAt ?? DateTime.now(),
        );
        n++;
      }
      if (!mounted) return;
      setState(() {
        _status = '$n Fotos in der Warteschlange (Notiz/Tags/Vorgang übernommen).';
        _clearPhotoOnly();
      });
      unawaited(queue.drain());
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Serie konnte nicht vollständig eingereiht werden.';
        _status = null;
      });
    } finally {
      if (mounted) setState(() => _enqueuing = false);
    }
  }

  Future<void> _openInWeb(RecentUpload item) async {
    final uri = widget.client.webRecordUri(item.id);
    try {
      final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!ok && mounted) {
        setState(() => _error = 'Konnte Web-UI nicht öffnen: $uri');
      }
    } catch (_) {
      if (mounted) setState(() => _error = 'Konnte Web-UI nicht öffnen: $uri');
    }
  }

  Future<void> _confirmLogout() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Abmelden?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Abbrechen')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Abmelden')),
        ],
      ),
    );
    if (ok == true) await widget.onLogout();
  }

  String _fmtTime(DateTime t) {
    final local = t.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(local.day)}.${two(local.month)}.${local.year} '
        '${two(local.hour)}:${two(local.minute)}:${two(local.second)}';
  }

  String _pendingStatusText(PendingUpload item) {
    switch (item.status) {
      case PendingUploadStatus.uploading:
        return 'Wird hochgeladen…';
      case PendingUploadStatus.failed:
        return item.lastError?.isNotEmpty == true
            ? item.lastError!
            : 'Wartet auf besseren Empfang';
      case PendingUploadStatus.pending:
        return 'Wartet';
    }
  }

  @override
  Widget build(BuildContext context) {
    final photo = _photo;
    final queue = _queue;
    final pending = queue?.items ?? const <PendingUpload>[];
    final draining = queue?.isDraining ?? false;
    final captureBusy = _enqueuing;
    final openCases = _cases;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Aufnahme'),
        actions: [
          IconButton(
            tooltip: 'Liste aktualisieren',
            onPressed: _loadingRecent
                ? null
                : () async {
                    await Future.wait([_refreshRecent(), _loadCases()]);
                    await _queue?.drain();
                  },
            icon: const Icon(Icons.refresh),
          ),
          IconButton(
            tooltip: 'Abmelden',
            onPressed: captureBusy ? null : _confirmLogout,
            icon: const Icon(Icons.logout),
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text(widget.settings.baseUrl, style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 12),
          AspectRatio(
            aspectRatio: 3 / 4,
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: Colors.black12,
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: Colors.black26),
              ),
              child: photo == null
                  ? const Center(child: Text('Noch kein Foto'))
                  : ClipRRect(
                      borderRadius: BorderRadius.circular(12),
                      child: Image.file(photo, fit: BoxFit.cover),
                    ),
            ),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: FilledButton.icon(
                  onPressed: captureBusy ? null : _takePhoto,
                  icon: const Icon(Icons.photo_camera),
                  label: Text(photo == null ? 'Aufnehmen' : 'Neu'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: captureBusy ? null : () => _pickGallery(),
                  icon: const Icon(Icons.photo_library),
                  label: const Text('Galerie'),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: captureBusy ? null : () => _pickGallery(multi: true),
                  icon: const Icon(Icons.collections),
                  label: const Text('Serie (Galerie)'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: FilterChip(
                  selected: _seriesMode,
                  onSelected: captureBusy
                      ? null
                      : (v) => setState(() => _seriesMode = v),
                  avatar: Icon(_seriesMode ? Icons.repeat_on : Icons.repeat),
                  label: const Text('Serie Kamera'),
                ),
              ),
            ],
          ),
          if (photo != null) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: captureBusy ? null : _pickMapPin,
                    icon: const Icon(Icons.map_outlined),
                    label: Text(_gpsKind == _GpsKind.none ? 'Ort auf Karte' : 'Ort ändern'),
                  ),
                ),
                if (_gpsKind != _GpsKind.device) ...[
                  const SizedBox(width: 8),
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: captureBusy ? null : _applyDeviceGpsOrWarn,
                      icon: const Icon(Icons.my_location),
                      label: const Text('Live-GPS'),
                    ),
                  ),
                ],
              ],
            ),
          ],
          const SizedBox(height: 12),
          TextField(
            controller: _noteCtrl,
            decoration: const InputDecoration(
              labelText: 'Notiz (optional)',
              alignLabelWithHint: true,
            ),
            minLines: 2,
            maxLines: 4,
            enabled: !captureBusy,
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _tagsCtrl,
            decoration: const InputDecoration(
              labelText: 'Tags (optional, Komma-getrennt)',
              hintText: 'z. B. tag, wand, bahnhof',
            ),
            enabled: !captureBusy,
            textInputAction: TextInputAction.done,
          ),
          const SizedBox(height: 12),
          InputDecorator(
            decoration: const InputDecoration(
              labelText: 'Vorgang (optional)',
              border: OutlineInputBorder(),
              filled: true,
              fillColor: Colors.white,
            ),
            child: DropdownButtonHideUnderline(
              child: DropdownButton<String?>(
                isExpanded: true,
                value: _caseId,
                hint: Text(
                  openCases.isEmpty ? 'Keine offenen Vorgänge' : 'Keiner',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                items: [
                  const DropdownMenuItem<String?>(
                    value: null,
                    child: Text('Keiner'),
                  ),
                  ...openCases.map(
                    (c) => DropdownMenuItem<String?>(
                      value: c.id,
                      child: Text(c.label, overflow: TextOverflow.ellipsis),
                    ),
                  ),
                ],
                onChanged: captureBusy ? null : (v) => setState(() => _caseId = v),
              ),
            ),
          ),
          if (_seriesMode) ...[
            const SizedBox(height: 8),
            Text(
              'Serie aktiv: nach dem Einreihen öffnet sich wieder die Kamera. Notiz, Tags und Vorgang bleiben — Upload läuft im Hintergrund.',
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ],
          if (_status != null) ...[
            const SizedBox(height: 12),
            Text(_status!),
          ],
          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          ],
          const SizedBox(height: 16),
          FilledButton.tonalIcon(
            onPressed: captureBusy || photo == null ? null : () => _enqueueUpload(),
            icon: _enqueuing
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.cloud_upload),
            label: const Text('Zur Warteschlange'),
          ),
          if (photo != null && !_seriesMode) ...[
            const SizedBox(height: 8),
            OutlinedButton.icon(
              onPressed: captureBusy ? null : () => _enqueueUpload(continueSeries: true),
              icon: const Icon(Icons.add_a_photo),
              label: const Text('Einreihen & weiter fotografieren'),
            ),
          ],
          if (pending.isNotEmpty) ...[
            const SizedBox(height: 28),
            Row(
              children: [
                Text('Warteschlange', style: Theme.of(context).textTheme.titleMedium),
                const Spacer(),
                if (draining)
                  const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                else
                  TextButton(
                    onPressed: () => _queue?.retryAll(),
                    child: const Text('Erneut senden'),
                  ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              '${pending.length} ausstehend — bei Netz wieder da automatisch erneut.',
              style: Theme.of(context).textTheme.bodySmall,
            ),
            const SizedBox(height: 8),
            ...pending.map((item) {
              final subtitle = [
                _fmtTime(item.createdAt),
                _pendingStatusText(item),
                if (item.tags.trim().isNotEmpty) item.tags.trim(),
                if (item.note.trim().isNotEmpty) item.note.trim(),
              ].join(' · ');
              return ListTile(
                contentPadding: EdgeInsets.zero,
                dense: true,
                leading: Icon(
                  item.status == PendingUploadStatus.uploading
                      ? Icons.cloud_upload
                      : item.status == PendingUploadStatus.failed
                          ? Icons.cloud_off
                          : Icons.cloud_queue,
                  color: item.status == PendingUploadStatus.failed
                      ? Theme.of(context).colorScheme.error
                      : Theme.of(context).colorScheme.primary,
                ),
                title: Text(
                  item.locationLabel,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                subtitle: Text(subtitle, maxLines: 2, overflow: TextOverflow.ellipsis),
                trailing: IconButton(
                  tooltip: 'Entfernen',
                  onPressed: item.status == PendingUploadStatus.uploading
                      ? null
                      : () => _queue?.remove(item.id),
                  icon: const Icon(Icons.delete_outline),
                ),
              );
            }),
          ],
          const SizedBox(height: 28),
          Row(
            children: [
              Text('Letzte Uploads', style: Theme.of(context).textTheme.titleMedium),
              const Spacer(),
              if (_loadingRecent)
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
            ],
          ),
          const SizedBox(height: 8),
          if (_recent.isEmpty)
            Text(
              'Noch keine Einträge.',
              style: Theme.of(context).textTheme.bodySmall,
            )
          else
            ..._recent.take(15).map((item) {
              final subtitle = [
                _fmtTime(item.uploadedAt),
                if (item.duplicate) 'Duplikat',
                if (item.note.trim().isNotEmpty) item.note.trim(),
              ].join(' · ');
              return ListTile(
                contentPadding: EdgeInsets.zero,
                dense: true,
                leading: Icon(
                  item.hasGps ? Icons.place : Icons.place_outlined,
                  color: item.hasGps
                      ? Theme.of(context).colorScheme.primary
                      : Theme.of(context).colorScheme.outline,
                ),
                title: Text(
                  item.locationLabel,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                subtitle: Text(subtitle, maxLines: 2, overflow: TextOverflow.ellipsis),
                trailing: IconButton(
                  tooltip: 'Im Web öffnen',
                  onPressed: () => _openInWeb(item),
                  icon: const Icon(Icons.open_in_browser),
                ),
                onTap: () => _openInWeb(item),
              );
            }),
        ],
      ),
    );
  }
}
