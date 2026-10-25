import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:geolocator/geolocator.dart';
import 'package:latlong2/latlong.dart';

/// Einfacher Karten-Pin: Tippen setzt den Ort, Übernehmen speichert.
class MapPinScreen extends StatefulWidget {
  const MapPinScreen({
    super.key,
    this.initialLat,
    this.initialLon,
  });

  final double? initialLat;
  final double? initialLon;

  @override
  State<MapPinScreen> createState() => _MapPinScreenState();
}

class _MapPinScreenState extends State<MapPinScreen> {
  // Köllnbrinkweg 6, 30966 Hemmingen (Nominatim)
  static const _fallback = LatLng(52.3249857, 9.7291676);
  static const _fallbackZoom = 17.0;
  static const _nearZero = 1e-6;

  late LatLng _pin;
  late double _zoom;
  late final MapController _map;
  bool _locating = false;

  bool get _hasUsefulInitial {
    final lat = widget.initialLat;
    final lon = widget.initialLon;
    if (lat == null || lon == null) return false;
    if (!lat.isFinite || !lon.isFinite) return false;
    // 0,0 ist praktisch nie ein sinnvoller Aufnahmeort.
    if (lat.abs() < _nearZero && lon.abs() < _nearZero) return false;
    return true;
  }

  @override
  void initState() {
    super.initState();
    _map = MapController();
    if (_hasUsefulInitial) {
      _pin = LatLng(widget.initialLat!, widget.initialLon!);
      _zoom = 17;
    } else {
      _pin = _fallback;
      _zoom = _fallbackZoom;
      _locating = true;
      WidgetsBinding.instance.addPostFrameCallback((_) => _bootstrapFromDeviceGps());
    }
  }

  Future<void> _bootstrapFromDeviceGps() async {
    try {
      final pos = await _readDeviceGps();
      if (!mounted || pos == null) return;
      final next = LatLng(pos.latitude, pos.longitude);
      setState(() {
        _pin = next;
        _zoom = 17;
        _locating = false;
      });
      _map.move(next, 17);
    } catch (_) {
      if (!mounted) return;
      setState(() => _locating = false);
    }
  }

  Future<Position?> _readDeviceGps() async {
    final enabled = await Geolocator.isLocationServiceEnabled();
    if (!enabled) return null;
    var permission = await Geolocator.checkPermission();
    if (permission == LocationPermission.denied) {
      permission = await Geolocator.requestPermission();
    }
    if (permission == LocationPermission.denied ||
        permission == LocationPermission.deniedForever) {
      return null;
    }
    return Geolocator.getCurrentPosition(
      locationSettings: const LocationSettings(
        accuracy: LocationAccuracy.high,
        timeLimit: Duration(seconds: 12),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Ort auf Karte'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, _pin),
            child: Text(
              'Übernehmen',
              style: TextStyle(color: Theme.of(context).colorScheme.onPrimary),
            ),
          ),
        ],
      ),
      body: Column(
        children: [
          Material(
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
              child: Text(
                _locating
                    ? 'Aktuellen Standort ermitteln…'
                    : 'Tippe auf die Karte, um den Aufnahmeort zu setzen.\n'
                        '${_pin.latitude.toStringAsFixed(5)}, ${_pin.longitude.toStringAsFixed(5)}',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
          ),
          Expanded(
            child: Stack(
              children: [
                FlutterMap(
                  mapController: _map,
                  options: MapOptions(
                    initialCenter: _pin,
                    initialZoom: _zoom,
                    onTap: (_, point) => setState(() => _pin = point),
                  ),
                  children: [
                    TileLayer(
                      urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
                      userAgentPackageName: 'de.schmutzfink.mobile',
                    ),
                    MarkerLayer(
                      markers: [
                        Marker(
                          point: _pin,
                          width: 40,
                          height: 40,
                          child: Icon(
                            Icons.place,
                            size: 40,
                            color: Theme.of(context).colorScheme.primary,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
                if (_locating)
                  const ColoredBox(
                    color: Color(0x66000000),
                    child: Center(child: CircularProgressIndicator()),
                  ),
              ],
            ),
          ),
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: FilledButton.icon(
                onPressed: _locating ? null : () => Navigator.pop(context, _pin),
                icon: const Icon(Icons.check),
                label: const Text('Diesen Ort verwenden'),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
