import 'package:flutter/material.dart';
import 'package:schmutzfink_mobile/app.dart';
import 'package:schmutzfink_mobile/api/client.dart';
import 'package:schmutzfink_mobile/settings.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final settings = await AppSettings.load();
  final client = ApiClient(settings: settings);
  runApp(SchmutzfinkApp(settings: settings, client: client));
}
