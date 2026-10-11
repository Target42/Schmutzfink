import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart';
import 'package:schmutzfink_mobile/api/models.dart';
import 'package:schmutzfink_mobile/settings.dart';

class ApiClient {
  ApiClient({required this.settings, http.Client? httpClient})
      : _http = httpClient ?? http.Client();

  final AppSettings settings;
  final http.Client _http;

  static const sessionCookie = 'sf_session';

  String? _session;

  bool get hasSession => _session != null && _session!.isNotEmpty;

  Future<void> restoreSession() async {
    _session = settings.sessionToken;
  }

  Future<void> clearSession() async {
    _session = null;
    await settings.setSessionToken(null);
  }

  Uri _uri(String path) {
    final base = settings.baseUrl;
    if (!path.startsWith('/')) path = '/$path';
    return Uri.parse('$base$path');
  }

  Map<String, String> _headers({bool jsonBody = false}) {
    final h = <String, String>{
      'Accept': 'application/json',
    };
    if (jsonBody) h['Content-Type'] = 'application/json; charset=utf-8';
    if (_session != null && _session!.isNotEmpty) {
      h['Cookie'] = '$sessionCookie=$_session';
    }
    return h;
  }

  Future<void> _rememberSessionFrom(http.BaseResponse res) async {
    final raw = res.headers['set-cookie'];
    if (raw == null || raw.isEmpty) return;
    final token = _parseCookie(raw, sessionCookie);
    if (token == null) return;
    if (token.isEmpty) {
      await clearSession();
      return;
    }
    _session = token;
    await settings.setSessionToken(token);
  }

  static String? _parseCookie(String setCookie, String name) {
    // Mehrere Cookies können kommen; wir suchen sf_session=…
    for (final part in setCookie.split(RegExp(r',(?=[^ ;]+=)'))) {
      final first = part.split(';').first.trim();
      final eq = first.indexOf('=');
      if (eq <= 0) continue;
      if (first.substring(0, eq) != name) continue;
      return first.substring(eq + 1);
    }
    // Fallback: einfacher Prefix
    final prefix = '$name=';
    final i = setCookie.indexOf(prefix);
    if (i < 0) return null;
    final rest = setCookie.substring(i + prefix.length);
    final end = rest.indexOf(';');
    return end < 0 ? rest : rest.substring(0, end);
  }

  Future<Map<String, dynamic>> _decode(http.Response res) async {
    await _rememberSessionFrom(res);
    if (res.statusCode == 401) {
      await clearSession();
      throw ApiException.fromBody(res.statusCode, res.body);
    }
    if (res.statusCode < 200 || res.statusCode >= 300) {
      throw ApiException.fromBody(res.statusCode, res.body);
    }
    if (res.body.isEmpty) return {};
    final decoded = jsonDecode(res.body);
    if (decoded is Map<String, dynamic>) return decoded;
    throw ApiException('Unerwartete Antwort');
  }

  Future<User> login(String username, String password) async {
    final res = await _http.post(
      _uri('/api/auth/login'),
      headers: _headers(jsonBody: true),
      body: jsonEncode({'username': username, 'password': password}),
    );
    final map = await _decode(res);
    final user = User.fromJson(map['user'] as Map<String, dynamic>);
    return user;
  }

  Future<void> logout() async {
    try {
      final res = await _http.post(_uri('/api/auth/logout'), headers: _headers());
      await _rememberSessionFrom(res);
    } finally {
      await clearSession();
    }
  }

  Future<User> me() async {
    final res = await _http.get(_uri('/api/auth/me'), headers: _headers());
    final map = await _decode(res);
    return User.fromJson(map['user'] as Map<String, dynamic>);
  }

  Future<User> changePassword({required String oldPassword, required String newPassword}) async {
    final res = await _http.post(
      _uri('/api/auth/password'),
      headers: _headers(jsonBody: true),
      body: jsonEncode({'old_password': oldPassword, 'new_password': newPassword}),
    );
    final map = await _decode(res);
    return User.fromJson(map['user'] as Map<String, dynamic>);
  }

  Future<List<RecentUpload>> listRecent({int limit = 15}) async {
    final res = await _http.get(
      _uri('/api/records?limit=$limit'),
      headers: _headers(),
    );
    final map = await _decode(res);
    final items = map['items'];
    if (items is! List) return [];
    return items
        .whereType<Map<String, dynamic>>()
        .map(RecentUpload.fromRecordJson)
        .where((e) => e.id.isNotEmpty)
        .toList();
  }

  Future<List<CaseSummary>> listCases() async {
    final res = await _http.get(_uri('/api/cases'), headers: _headers());
    final map = await _decode(res);
    final items = map['items'];
    if (items is! List) return [];
    return items
        .whereType<Map<String, dynamic>>()
        .map(CaseSummary.fromJson)
        .where((e) => e.id.isNotEmpty)
        .toList();
  }

  /// Web-UI und API laufen auf demselben Host — Deep-Link zum Datensatz.
  Uri webRecordUri(String recordId) {
    final id = recordId.trim();
    return _uri('/datensatz/$id');
  }

  Future<UploadResult> uploadPhoto({
    required File file,
    double? lat,
    double? lon,
    String? note,
    String? tags,
    String? caseId,
    DateTime? capturedAt,
  }) async {
    final req = http.MultipartRequest('POST', _uri('/api/records'));
    req.headers.addAll(_headers());
    final name = file.uri.pathSegments.isNotEmpty ? file.uri.pathSegments.last : 'capture.jpg';
    req.files.add(
      await http.MultipartFile.fromPath(
        'file',
        file.path,
        contentType: _imageMediaType(name),
        filename: name,
      ),
    );
    if (lat != null && lon != null) {
      req.fields['lat'] = lat.toString();
      req.fields['lon'] = lon.toString();
    }
    final trimmedNote = note?.trim() ?? '';
    if (trimmedNote.isNotEmpty) {
      req.fields['note'] = trimmedNote;
    }
    final trimmedTags = tags?.trim() ?? '';
    if (trimmedTags.isNotEmpty) {
      req.fields['tags'] = trimmedTags;
    }
    final trimmedCase = caseId?.trim() ?? '';
    if (trimmedCase.isNotEmpty) {
      req.fields['case_id'] = trimmedCase;
    }
    if (capturedAt != null) {
      req.fields['captured_at'] = capturedAt.toUtc().toIso8601String();
    }

    final streamed = await _http.send(req).timeout(const Duration(seconds: 90));
    final res = await http.Response.fromStream(streamed).timeout(const Duration(seconds: 90));
    await _rememberSessionFrom(res);

    if (res.statusCode == 401) {
      await clearSession();
      throw ApiException.fromBody(res.statusCode, res.body);
    }
    if (res.statusCode == 409) {
      final err = ApiException.fromBody(res.statusCode, res.body);
      final id = err.duplicateId ?? '';
      if (id.isNotEmpty) {
        return UploadResult(
          id: id,
          duplicate: true,
          hasGps: lat != null && lon != null,
          uploadedAt: DateTime.now(),
          note: trimmedNote,
          lat: lat,
          lon: lon,
        );
      }
      throw err;
    }
    if (res.statusCode == 403) {
      throw ApiException.fromBody(res.statusCode, res.body);
    }
    if (res.statusCode < 200 || res.statusCode >= 300) {
      throw ApiException.fromBody(res.statusCode, res.body);
    }
    final map = jsonDecode(res.body) as Map<String, dynamic>;
    final id = map['id'] as String? ?? '';
    if (id.isEmpty) throw ApiException('Upload ohne ID');
    final outLat = asDouble(map['lat']) ?? lat;
    final outLon = asDouble(map['lon']) ?? lon;
    return UploadResult(
      id: id,
      hasGps: map['has_gps'] == true || (outLat != null && outLon != null),
      uploadedAt: DateTime.tryParse(map['uploaded_at'] as String? ?? '')?.toLocal() ?? DateTime.now(),
      note: trimmedNote,
      lat: outLat,
      lon: outLon,
      addressLabel: addressLabelFromJson(map['address']) ?? '',
    );
  }
}

MediaType _imageMediaType(String filename) {
  final lower = filename.toLowerCase();
  if (lower.endsWith('.png')) return MediaType('image', 'png');
  if (lower.endsWith('.webp')) return MediaType('image', 'webp');
  if (lower.endsWith('.gif')) return MediaType('image', 'gif');
  return MediaType('image', 'jpeg');
}
