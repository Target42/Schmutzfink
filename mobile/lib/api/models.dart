import 'dart:convert';

class User {
  User({
    required this.id,
    required this.username,
    required this.role,
    required this.mustChangePassword,
    required this.canWrite,
  });

  final String id;
  final String username;
  final String role;
  final bool mustChangePassword;
  final bool canWrite;

  factory User.fromJson(Map<String, dynamic> json) {
    final role = (json['role'] as String?) ?? 'user';
    return User(
      id: json['id'] as String? ?? '',
      username: json['username'] as String? ?? '',
      role: role,
      mustChangePassword: json['must_change_password'] == true,
      canWrite: role == 'admin' || role == 'user',
    );
  }
}

/// Offener oder geschlossener Vorgang aus `GET /api/cases`.
class CaseSummary {
  CaseSummary({
    required this.id,
    required this.title,
    required this.kind,
    this.closedAt,
    this.photoCount = 0,
  });

  final String id;
  final String title;
  final String kind;
  final DateTime? closedAt;
  final int photoCount;

  bool get isOpen => closedAt == null;

  String get kindLabel => kind == 'criminal' ? 'Strafrechtlich' : 'Zivilrechtlich';

  String get label {
    final base = title.trim().isEmpty ? 'Ohne Titel' : title.trim();
    return '$base · $kindLabel';
  }

  factory CaseSummary.fromJson(Map<String, dynamic> json) {
    return CaseSummary(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      kind: json['kind'] as String? ?? 'civil',
      closedAt: DateTime.tryParse(json['closed_at'] as String? ?? ''),
      photoCount: json['photo_count'] as int? ?? 0,
    );
  }
}

class UploadResult {
  UploadResult({
    required this.id,
    this.duplicate = false,
    this.hasGps = false,
    this.uploadedAt,
    this.note = '',
    this.lat,
    this.lon,
    this.addressLabel = '',
  });

  final String id;
  final bool duplicate;
  final bool hasGps;
  final DateTime? uploadedAt;
  final String note;
  final double? lat;
  final double? lon;
  final String addressLabel;

  String get locationLabel => formatLocationLabel(
        addressLabel: addressLabel,
        lat: lat,
        lon: lon,
        hasGps: hasGps,
      );
}

class RecentUpload {
  RecentUpload({
    required this.id,
    required this.uploadedAt,
    this.note = '',
    this.hasGps = false,
    this.duplicate = false,
    this.source = 'local',
    this.lat,
    this.lon,
    this.addressLabel = '',
  });

  final String id;
  final DateTime uploadedAt;
  final String note;
  final bool hasGps;
  final bool duplicate;
  final String source; // local | server
  final double? lat;
  final double? lon;
  final String addressLabel;

  String get locationLabel => formatLocationLabel(
        addressLabel: addressLabel,
        lat: lat,
        lon: lon,
        hasGps: hasGps,
      );

  factory RecentUpload.fromJson(Map<String, dynamic> json) {
    final lat = asDouble(json['lat']);
    final lon = asDouble(json['lon']);
    return RecentUpload(
      id: json['id'] as String? ?? '',
      uploadedAt: DateTime.tryParse(json['uploaded_at'] as String? ?? '') ?? DateTime.now(),
      note: json['note'] as String? ?? '',
      hasGps: json['has_gps'] == true || (lat != null && lon != null),
      duplicate: json['duplicate'] == true,
      source: json['source'] as String? ?? 'local',
      lat: lat,
      lon: lon,
      addressLabel: addressLabelFromJson(json['address']) ?? (json['address_label'] as String? ?? ''),
    );
  }

  factory RecentUpload.fromRecordJson(Map<String, dynamic> json) {
    final lat = asDouble(json['lat']);
    final lon = asDouble(json['lon']);
    var note = (json['note'] as String?)?.trim() ?? '';
    if (note.isEmpty) {
      final sightings = json['sightings'];
      if (sightings is List && sightings.isNotEmpty) {
        final first = sightings.first;
        if (first is Map<String, dynamic>) {
          note = first['note'] as String? ?? '';
        }
      }
    }
    return RecentUpload(
      id: (json['record_id'] as String?)?.isNotEmpty == true
          ? json['record_id'] as String
          : (json['id'] as String? ?? ''),
      uploadedAt: DateTime.tryParse(json['uploaded_at'] as String? ?? '')?.toLocal() ?? DateTime.now(),
      note: note,
      hasGps: json['has_gps'] == true || (lat != null && lon != null),
      source: 'server',
      lat: lat,
      lon: lon,
      addressLabel: addressLabelFromJson(json['address']) ?? '',
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'uploaded_at': uploadedAt.toIso8601String(),
        'note': note,
        'has_gps': hasGps,
        'duplicate': duplicate,
        'source': source,
        if (lat != null) 'lat': lat,
        if (lon != null) 'lon': lon,
        if (addressLabel.isNotEmpty) 'address_label': addressLabel,
      };
}

String formatLocationLabel({
  String addressLabel = '',
  double? lat,
  double? lon,
  bool hasGps = false,
}) {
  final trimmed = addressLabel.trim();
  if (trimmed.isNotEmpty) return trimmed;
  if (lat != null && lon != null) {
    return '${lat.toStringAsFixed(5)}, ${lon.toStringAsFixed(5)}';
  }
  if (hasGps) return 'GPS';
  return 'ohne GPS';
}

String? addressLabelFromJson(dynamic raw) {
  if (raw is! Map) return null;
  final map = Map<String, dynamic>.from(raw);
  final label = (map['label'] as String?)?.trim() ?? '';
  if (label.isNotEmpty) return label;
  final street = (map['street'] as String?)?.trim() ?? '';
  final city = (map['city'] as String?)?.trim() ?? '';
  final postal = (map['postal_code'] as String?)?.trim() ?? '';
  final loc = [
    if (postal.isNotEmpty && city.isNotEmpty) '$postal $city' else if (city.isNotEmpty) city else postal,
  ].join();
  if (street.isNotEmpty && loc.isNotEmpty) return '$street, $loc';
  if (street.isNotEmpty) return street;
  if (loc.isNotEmpty) return loc;
  return null;
}

double? asDouble(dynamic raw) {
  if (raw is num) return raw.toDouble();
  if (raw is String) return double.tryParse(raw);
  return null;
}

class ApiException implements Exception {
  ApiException(this.message, {this.statusCode, this.code, this.duplicateId});

  final String message;
  final int? statusCode;
  final String? code;
  final String? duplicateId;

  @override
  String toString() => message;

  factory ApiException.fromBody(int status, String body) {
    try {
      final map = jsonDecode(body) as Map<String, dynamic>;
      return ApiException(
        (map['error'] as String?) ?? 'Anfrage fehlgeschlagen ($status)',
        statusCode: status,
        code: map['code'] as String?,
        duplicateId: (map['record_id'] as String?) ?? (map['id'] as String?),
      );
    } catch (_) {
      return ApiException('Anfrage fehlgeschlagen ($status)', statusCode: status);
    }
  }
}
