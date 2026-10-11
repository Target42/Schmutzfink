import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:schmutzfink_mobile/api/models.dart';

typedef UploadSender = Future<UploadResult> Function(PendingUpload item);

/// Persistierte Upload-Warteschlange für schlechten Empfang.
class UploadQueue {
  UploadQueue(this._prefs, {required this.send});

  final SharedPreferences _prefs;
  final UploadSender send;

  static const _key = 'upload_queue_v1';

  List<PendingUpload> _items = [];
  bool _draining = false;
  void Function()? onChanged;
  void Function(RecentUpload entry)? onUploaded;
  void Function(ApiException err)? onAuthLost;

  List<PendingUpload> get items => List.unmodifiable(_items);
  bool get isDraining => _draining;
  int get pendingCount => _items.length;

  Future<void> load() async {
    final raw = _prefs.getString(_key);
    if (raw == null || raw.isEmpty) {
      _items = [];
      return;
    }
    try {
      final list = jsonDecode(raw) as List<dynamic>;
      _items = list
          .whereType<Map<String, dynamic>>()
          .map(PendingUpload.fromJson)
          .where((e) => e.filePath.isNotEmpty)
          .toList();
    } catch (_) {
      _items = [];
    }
  }

  Future<PendingUpload> enqueue({
    required File file,
    double? lat,
    double? lon,
    String note = '',
    String tags = '',
    String caseId = '',
    DateTime? capturedAt,
  }) async {
    final id = _newId();
    final stored = await _copyIntoQueueDir(file, id);
    final item = PendingUpload(
      id: id,
      filePath: stored.path,
      lat: lat,
      lon: lon,
      note: note.trim(),
      tags: tags.trim(),
      caseId: caseId.trim(),
      capturedAt: capturedAt,
      createdAt: DateTime.now(),
    );
    _items = [..._items, item];
    await _persist();
    _notify();
    return item;
  }

  Future<void> remove(String id) async {
    final match = _items.where((e) => e.id == id).toList();
    _items = _items.where((e) => e.id != id).toList();
    await _persist();
    for (final item in match) {
      await _deleteFileQuietly(item.filePath);
    }
    _notify();
  }

  Future<void> drain() async {
    if (_draining) return;
    if (_items.isEmpty) return;
    _draining = true;
    _notify();
    try {
      while (_items.isNotEmpty) {
        final current = _items.first;
        final working = current.copyWith(
          status: PendingUploadStatus.uploading,
          clearError: true,
        );
        _replace(working);
        await _persist();
        _notify();

        try {
          final result = await send(working);
          final entry = RecentUpload(
            id: result.id,
            uploadedAt: result.uploadedAt ?? DateTime.now(),
            note: result.note.isNotEmpty ? result.note : working.note,
            hasGps: result.hasGps || (working.lat != null && working.lon != null),
            duplicate: result.duplicate,
            source: 'local',
            lat: result.lat ?? working.lat,
            lon: result.lon ?? working.lon,
            addressLabel: result.addressLabel,
          );
          await remove(working.id);
          onUploaded?.call(entry);
        } on ApiException catch (e) {
          if (e.statusCode == 401) {
            _replace(working.copyWith(
              status: PendingUploadStatus.failed,
              attempts: working.attempts + 1,
              lastError: e.message,
            ));
            await _persist();
            onAuthLost?.call(e);
            break;
          }
          // 4xx außer Auth: nicht endlos wiederholen (z. B. ungültiger Vorgang).
          if (e.statusCode != null && e.statusCode! >= 400 && e.statusCode! < 500) {
            _replace(working.copyWith(
              status: PendingUploadStatus.failed,
              attempts: working.attempts + 1,
              lastError: e.message,
            ));
            await _persist();
            _notify();
            break;
          }
          _replace(working.copyWith(
            status: PendingUploadStatus.failed,
            attempts: working.attempts + 1,
            lastError: e.message,
          ));
          await _persist();
          _notify();
          break;
        } catch (_) {
          _replace(working.copyWith(
            status: PendingUploadStatus.failed,
            attempts: working.attempts + 1,
            lastError: 'Netzwerkfehler — wird erneut versucht.',
          ));
          await _persist();
          _notify();
          break;
        }
      }
    } finally {
      _draining = false;
      _notify();
    }
  }

  /// Setzt Fehlermeldungen zurück und startet die Queue erneut.
  Future<void> retryAll() async {
    _items = [
      for (final e in _items)
        e.copyWith(
          attempts: 0,
          status: PendingUploadStatus.pending,
          clearError: true,
        ),
    ];
    await _persist();
    _notify();
    await drain();
  }

  void _replace(PendingUpload item) {
    _items = [for (final e in _items) e.id == item.id ? item : e];
  }

  Future<void> _persist() async {
    await _prefs.setString(
      _key,
      jsonEncode(_items.map((e) => e.toJson()).toList()),
    );
  }

  void _notify() => onChanged?.call();

  static String _newId() {
    final rnd = Random();
    final stamp = DateTime.now().toUtc().microsecondsSinceEpoch;
    return '${stamp}_${rnd.nextInt(1 << 32).toRadixString(16)}';
  }

  static Future<File> _copyIntoQueueDir(File source, String id) async {
    final root = await getApplicationDocumentsDirectory();
    final dir = Directory(p.join(root.path, 'upload_queue'));
    if (!await dir.exists()) {
      await dir.create(recursive: true);
    }
    final ext = p.extension(source.path).toLowerCase();
    final safeExt = ext.isEmpty ? '.jpg' : ext;
    final dest = File(p.join(dir.path, '$id$safeExt'));
    return source.copy(dest.path);
  }

  static Future<void> _deleteFileQuietly(String path) async {
    try {
      final f = File(path);
      if (await f.exists()) await f.delete();
    } catch (_) {}
  }
}

enum PendingUploadStatus { pending, uploading, failed }

class PendingUpload {
  PendingUpload({
    required this.id,
    required this.filePath,
    this.lat,
    this.lon,
    this.note = '',
    this.tags = '',
    this.caseId = '',
    this.capturedAt,
    DateTime? createdAt,
    this.attempts = 0,
    this.lastError,
    this.status = PendingUploadStatus.pending,
  }) : createdAt = createdAt ?? DateTime.now();

  final String id;
  final String filePath;
  final double? lat;
  final double? lon;
  final String note;
  final String tags;
  final String caseId;
  final DateTime? capturedAt;
  final DateTime createdAt;
  final int attempts;
  final String? lastError;
  final PendingUploadStatus status;

  String get locationLabel => formatLocationLabel(
        lat: lat,
        lon: lon,
        hasGps: lat != null && lon != null,
      );

  PendingUpload copyWith({
    String? id,
    String? filePath,
    double? lat,
    double? lon,
    String? note,
    String? tags,
    String? caseId,
    DateTime? capturedAt,
    DateTime? createdAt,
    int? attempts,
    String? lastError,
    bool clearError = false,
    PendingUploadStatus? status,
  }) {
    return PendingUpload(
      id: id ?? this.id,
      filePath: filePath ?? this.filePath,
      lat: lat ?? this.lat,
      lon: lon ?? this.lon,
      note: note ?? this.note,
      tags: tags ?? this.tags,
      caseId: caseId ?? this.caseId,
      capturedAt: capturedAt ?? this.capturedAt,
      createdAt: createdAt ?? this.createdAt,
      attempts: attempts ?? this.attempts,
      lastError: clearError ? null : (lastError ?? this.lastError),
      status: status ?? this.status,
    );
  }

  factory PendingUpload.fromJson(Map<String, dynamic> json) {
    return PendingUpload(
      id: json['id'] as String? ?? '',
      filePath: json['file_path'] as String? ?? '',
      lat: asDouble(json['lat']),
      lon: asDouble(json['lon']),
      note: json['note'] as String? ?? '',
      tags: json['tags'] as String? ?? '',
      caseId: json['case_id'] as String? ?? '',
      capturedAt: DateTime.tryParse(json['captured_at'] as String? ?? ''),
      createdAt: DateTime.tryParse(json['created_at'] as String? ?? '') ?? DateTime.now(),
      attempts: json['attempts'] as int? ?? 0,
      lastError: json['last_error'] as String?,
      status: PendingUploadStatus.values.firstWhere(
        (e) => e.name == (json['status'] as String? ?? ''),
        orElse: () => PendingUploadStatus.pending,
      ),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'file_path': filePath,
        if (lat != null) 'lat': lat,
        if (lon != null) 'lon': lon,
        'note': note,
        'tags': tags,
        'case_id': caseId,
        if (capturedAt != null) 'captured_at': capturedAt!.toUtc().toIso8601String(),
        'created_at': createdAt.toUtc().toIso8601String(),
        'attempts': attempts,
        if (lastError != null) 'last_error': lastError,
        'status': status.name,
      };
}
