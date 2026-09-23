import 'package:flutter/material.dart';
import 'package:schmutzfink_mobile/api/client.dart';
import 'package:schmutzfink_mobile/api/models.dart';
import 'package:schmutzfink_mobile/screens/password_screen.dart';
import 'package:schmutzfink_mobile/settings.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({
    super.key,
    required this.client,
    required this.settings,
    required this.onLoggedIn,
  });

  final ApiClient client;
  final AppSettings settings;
  final VoidCallback onLoggedIn;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _userCtrl = TextEditingController();
  final _passCtrl = TextEditingController();
  final _urlCtrl = TextEditingController();
  bool _busy = false;
  bool _showPassword = false;
  bool _showServer = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _urlCtrl.text = widget.settings.baseUrl;
    // Beim ersten Start (keine URL) Server-Feld direkt öffnen.
    _showServer = _urlCtrl.text.trim().isEmpty;
  }

  @override
  void dispose() {
    _userCtrl.dispose();
    _passCtrl.dispose();
    _urlCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final url = _urlCtrl.text.trim();
    if (url.isEmpty) {
      setState(() {
        _error = 'Bitte Server-URL einstellen.';
        _showServer = true;
      });
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await widget.settings.setBaseUrl(url);
      final user = await widget.client.login(_userCtrl.text.trim(), _passCtrl.text);
      if (!user.canWrite) {
        await widget.client.logout();
        throw ApiException('Dieses Konto darf keine Fotos hochladen (Rolle Sucher).');
      }
      if (!mounted) return;
      if (user.mustChangePassword) {
        final changed = await Navigator.of(context).push<bool>(
          MaterialPageRoute(
            builder: (_) => PasswordScreen(client: widget.client, forced: true),
          ),
        );
        if (changed != true) {
          await widget.client.logout();
          throw ApiException('Passwort muss geändert werden, bevor die App nutzbar ist.');
        }
      }
      widget.onLoggedIn();
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (e) {
      setState(() => _error = 'Server nicht erreichbar. URL prüfen.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(24, 48, 24, 24),
          children: [
            Center(
              child: Image.asset(
                'assets/logo.png',
                width: 168,
                height: 168,
                semanticLabel: 'Schmutzfink',
              ),
            ),
            const SizedBox(height: 20),
            Text(
              'Schmutzfink',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.headlineLarge?.copyWith(
                    fontWeight: FontWeight.w700,
                    color: Theme.of(context).colorScheme.primary,
                  ),
            ),
            const SizedBox(height: 8),
            Text(
              'Vor Ort fotografieren — GPS kommt direkt vom Gerät.',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyLarge,
            ),
            const SizedBox(height: 32),
            TextField(
              controller: _userCtrl,
              decoration: const InputDecoration(labelText: 'Benutzername'),
              textInputAction: TextInputAction.next,
              autocorrect: false,
              enabled: !_busy,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _passCtrl,
              decoration: InputDecoration(
                labelText: 'Passwort',
                suffixIcon: IconButton(
                  tooltip: _showPassword ? 'Passwort verbergen' : 'Passwort anzeigen',
                  onPressed: _busy
                      ? null
                      : () => setState(() => _showPassword = !_showPassword),
                  icon: Icon(_showPassword ? Icons.visibility_off : Icons.visibility),
                ),
              ),
              obscureText: !_showPassword,
              onSubmitted: (_) => _busy ? null : _submit(),
              enabled: !_busy,
            ),
            const SizedBox(height: 8),
            TextButton(
              onPressed: _busy ? null : () => setState(() => _showServer = !_showServer),
              child: Text(_showServer ? 'Server ausblenden' : 'Server einstellen'),
            ),
            if (_showServer) ...[
              TextField(
                controller: _urlCtrl,
                decoration: const InputDecoration(
                  labelText: 'API-Basis-URL',
                  hintText: 'http://10.0.2.2:8787',
                  helperText: 'Emulator: 10.0.2.2 — Gerät: LAN-IP des Servers',
                ),
                keyboardType: TextInputType.url,
                autocorrect: false,
                enabled: !_busy,
              ),
              const SizedBox(height: 8),
            ],
            if (_error != null) ...[
              Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
              const SizedBox(height: 12),
            ],
            FilledButton(
              onPressed: _busy ? null : _submit,
              child: _busy
                  ? const SizedBox(
                      height: 22,
                      width: 22,
                      child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                    )
                  : const Text('Anmelden'),
            ),
          ],
        ),
      ),
    );
  }
}
