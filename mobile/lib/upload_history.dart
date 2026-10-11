import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';
import 'package:schmutzfink_mobile/api/models.dart';

class UploadHistory {
  UploadHistory(this._prefs);

  final SharedPreferences _prefs;
  static const _key = 'recent_uploads';
  static const _max = 20;

  List<RecentUpload> load() {
    final raw = _prefs.getString(_key);
    if (raw == null || raw.isEmpty) return [];
    try {
      final list = jsonDecode(raw) as List<dynamic>;
      return list
          .whereType<Map<String, dynamic>>()
          .map(RecentUpload.fromJson)
          .toList();
    } catch (_) {
      return [];
    }
  }

  Future<void> prepend(RecentUpload item) async {
    final next = [item, ...load().where((e) => e.id != item.id)].take(_max).toList();
    await _prefs.setString(_key, jsonEncode(next.map((e) => e.toJson()).toList()));
  }

  Future<void> clear() async => _prefs.remove(_key);
}
