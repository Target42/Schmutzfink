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

class FieldDef {
  FieldDef({
    required this.id,
    required this.key,
    required this.label,
    required this.type,
    required this.options,
    required this.requiredField,
  });

  final String id;
  final String key;
  final String label;
  final String type;
  final List<String> options;
  final bool requiredField;

  factory FieldDef.fromJson(Map<String, dynamic> json) {
    final raw = json['options'];
    final options = <String>[];
    if (raw is List) {
      for (final item in raw) {
        final text = item?.toString().trim() ?? '';
        if (text.isNotEmpty) options.add(text);
      }
    }
    return FieldDef(
      id: json['id'] as String? ?? '',
      key: json['key'] as String? ?? '',
      label: json['label'] as String? ?? '',
      type: json['type'] as String? ?? 'text',
      options: options,
      requiredField: json['required'] == true,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'key': key,
        'label': label,
        'type': type,
        'options': options,
        'required': requiredField,
      };
}

class Roi {
  const Roi({required this.x, required this.y, required this.w, required this.h});

  final double x;
  final double y;
  final double w;
  final double h;

  Map<String, dynamic> toJson() => {'x': x, 'y': y, 'w': w, 'h': h};

  factory Roi.fromJson(Map<String, dynamic> json) {
    return Roi(
      x: asDouble(json['x']) ?? 0,
      y: asDouble(json['y']) ?? 0,
      w: asDouble(json['w']) ?? 0,
      h: asDouble(json['h']) ?? 0,
    );
  }
}

class NearbyPlace {
  NearbyPlace({
    required this.key,
    required this.title,
    required this.lastAt,
    required this.distanceM,
    required this.count,
    required this.sightingId,
    this.motifId = '',
    this.caseId = '',
    this.caseTitle = '',
    this.thumbUrl = '',
    this.followUp = false,
  });

  final String key;
  final String motifId;
  final String title;
  final String caseId;
  final String caseTitle;
  final DateTime lastAt;
  final double distanceM;
  final int count;
  final String sightingId;
  final String thumbUrl;
  final bool followUp;

  factory NearbyPlace.fromJson(Map<String, dynamic> json) {
    return NearbyPlace(
      key: json['key'] as String? ?? '',
      motifId: json['motif_id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      caseId: json['case_id'] as String? ?? '',
      caseTitle: json['case_title'] as String? ?? '',
      lastAt: DateTime.tryParse(json['last_at'] as String? ?? '') ?? DateTime.now(),
      distanceM: asDouble(json['distance_m']) ?? 0,
      count: json['count'] as int? ?? 1,
      sightingId: json['sighting_id'] as String? ?? '',
      thumbUrl: json['thumb_url'] as String? ?? '',
      followUp: json['follow_up'] == true,
    );
  }
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
