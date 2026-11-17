import 'dart:io';

import 'package:exif/exif.dart';

class ExifMeta {
  const ExifMeta({this.lat, this.lon, this.capturedAt});

  final double? lat;
  final double? lon;
  final DateTime? capturedAt;

  bool get hasGps => lat != null && lon != null;
}

/// Liest GPS und Aufnahmezeit aus JPEG-EXIF. Andere Formate: leeres Ergebnis.
Future<ExifMeta> readExifMeta(File file) async {
  try {
    final bytes = await file.readAsBytes();
    final tags = await readExifFromBytes(bytes);
    if (tags.isEmpty) return const ExifMeta();

    final lat = _gpsCoord(tags, 'GPS GPSLatitude', 'GPS GPSLatitudeRef');
    final lon = _gpsCoord(tags, 'GPS GPSLongitude', 'GPS GPSLongitudeRef');
    final capturedAt = _parseDate(tags['Image DateTime']?.printable) ??
        _parseDate(tags['EXIF DateTimeOriginal']?.printable);

    if (lat == null || lon == null) {
      return ExifMeta(capturedAt: capturedAt);
    }
    if (lat.isNaN || lon.isNaN || lat < -90 || lat > 90 || lon < -180 || lon > 180) {
      return ExifMeta(capturedAt: capturedAt);
    }
    return ExifMeta(lat: lat, lon: lon, capturedAt: capturedAt);
  } catch (_) {
    return const ExifMeta();
  }
}

double? _gpsCoord(Map<String, IfdTag> tags, String valueKey, String refKey) {
  final values = tags[valueKey]?.values;
  if (values == null) return null;
  final parts = values.toList();
  if (parts.length < 3) return null;
  double? asDouble(dynamic v) {
    if (v is Ratio) return v.toDouble();
    if (v is num) return v.toDouble();
    return double.tryParse(v.toString());
  }

  final deg = asDouble(parts[0]);
  final min = asDouble(parts[1]);
  final sec = asDouble(parts[2]);
  if (deg == null || min == null || sec == null) return null;
  var dec = deg + (min / 60.0) + (sec / 3600.0);
  final ref = tags[refKey]?.printable.toUpperCase() ?? '';
  if (ref == 'S' || ref == 'W') dec = -dec;
  return dec;
}

DateTime? _parseDate(String? raw) {
  if (raw == null || raw.isEmpty) return null;
  // EXIF: "yyyy:MM:dd HH:mm:ss"
  final m = RegExp(r'^(\d{4}):(\d{2}):(\d{2})[ T](\d{2}):(\d{2}):(\d{2})').firstMatch(raw);
  if (m == null) return null;
  try {
    return DateTime(
      int.parse(m.group(1)!),
      int.parse(m.group(2)!),
      int.parse(m.group(3)!),
      int.parse(m.group(4)!),
      int.parse(m.group(5)!),
      int.parse(m.group(6)!),
    );
  } catch (_) {
    return null;
  }
}
