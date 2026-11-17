import 'package:shared_preferences/shared_preferences.dart';

class AppSettings {
  AppSettings._(this._prefs);

  final SharedPreferences _prefs;

  static const _baseUrlKey = 'base_url';
  static const _sessionKey = 'sf_session';

  /// Optionaler Build-Default; sonst leer (Nutzer stellt die URL beim ersten Login ein).
  /// Beispiel: --dart-define=DEFAULT_BASE_URL=http://10.0.2.2:8787
  static String defaultBaseUrl() {
    const fromDefine = String.fromEnvironment('DEFAULT_BASE_URL');
    if (fromDefine.trim().isNotEmpty) {
      return fromDefine.trim().replaceAll(RegExp(r'/+$'), '');
    }
    return '';
  }

  static Future<AppSettings> load() async {
    final prefs = await SharedPreferences.getInstance();
    return AppSettings._(prefs);
  }

  String get baseUrl {
    final raw = (_prefs.getString(_baseUrlKey) ?? '').trim();
    if (raw.isEmpty) return defaultBaseUrl();
    return raw.replaceAll(RegExp(r'/+$'), '');
  }

  Future<void> setBaseUrl(String value) async {
    final cleaned = value.trim().replaceAll(RegExp(r'/+$'), '');
    await _prefs.setString(_baseUrlKey, cleaned);
  }

  String? get sessionToken {
    final v = _prefs.getString(_sessionKey);
    if (v == null || v.isEmpty) return null;
    return v;
  }

  Future<void> setSessionToken(String? token) async {
    if (token == null || token.isEmpty) {
      await _prefs.remove(_sessionKey);
    } else {
      await _prefs.setString(_sessionKey, token);
    }
  }
}
