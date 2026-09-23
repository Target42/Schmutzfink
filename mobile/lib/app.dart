import 'package:flutter/material.dart';
import 'package:schmutzfink_mobile/api/client.dart';
import 'package:schmutzfink_mobile/api/models.dart';
import 'package:schmutzfink_mobile/screens/capture_screen.dart';
import 'package:schmutzfink_mobile/screens/login_screen.dart';
import 'package:schmutzfink_mobile/settings.dart';
import 'package:schmutzfink_mobile/theme.dart';

class SchmutzfinkApp extends StatefulWidget {
  const SchmutzfinkApp({super.key, required this.settings, required this.client});

  final AppSettings settings;
  final ApiClient client;

  @override
  State<SchmutzfinkApp> createState() => _SchmutzfinkAppState();
}

class _SchmutzfinkAppState extends State<SchmutzfinkApp> {
  bool _booting = true;
  bool _loggedIn = false;

  @override
  void initState() {
    super.initState();
    _restoreSession();
  }

  Future<void> _restoreSession() async {
    await widget.client.restoreSession();
    var ok = false;
    if (widget.client.hasSession) {
      try {
        await widget.client.me();
        ok = true;
      } on ApiException {
        await widget.client.clearSession();
      } catch (_) {
        // Offline oder Server unerreichbar — Login zeigen.
      }
    }
    if (!mounted) return;
    setState(() {
      _loggedIn = ok;
      _booting = false;
    });
  }

  void _onLoggedIn() {
    setState(() => _loggedIn = true);
  }

  Future<void> _onLogout() async {
    try {
      await widget.client.logout();
    } catch (_) {
      await widget.client.clearSession();
    }
    if (!mounted) return;
    setState(() => _loggedIn = false);
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Schmutzfink',
      theme: schmutzfinkTheme(),
      home: _booting
          ? const Scaffold(body: Center(child: CircularProgressIndicator()))
          : _loggedIn
              ? CaptureScreen(client: widget.client, settings: widget.settings, onLogout: _onLogout)
              : LoginScreen(
                  client: widget.client,
                  settings: widget.settings,
                  onLoggedIn: _onLoggedIn,
                ),
    );
  }
}
