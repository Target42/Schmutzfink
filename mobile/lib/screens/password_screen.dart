import 'package:flutter/material.dart';
import 'package:schmutzfink_mobile/api/client.dart';
import 'package:schmutzfink_mobile/api/models.dart';

class PasswordScreen extends StatefulWidget {
  const PasswordScreen({super.key, required this.client, this.forced = false});

  final ApiClient client;
  final bool forced;

  @override
  State<PasswordScreen> createState() => _PasswordScreenState();
}

class _PasswordScreenState extends State<PasswordScreen> {
  final _oldCtrl = TextEditingController();
  final _newCtrl = TextEditingController();
  final _repeatCtrl = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _oldCtrl.dispose();
    _newCtrl.dispose();
    _repeatCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_newCtrl.text != _repeatCtrl.text) {
      setState(() => _error = 'Neues Passwort stimmt nicht überein.');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await widget.client.changePassword(
        oldPassword: _oldCtrl.text,
        newPassword: _newCtrl.text,
      );
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Passwort konnte nicht geändert werden.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Passwort ändern'),
        automaticallyImplyLeading: !widget.forced,
      ),
      body: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          if (widget.forced)
            const Padding(
              padding: EdgeInsets.only(bottom: 16),
              child: Text('Bitte setzen Sie ein eigenes Passwort, bevor Sie die App nutzen.'),
            ),
          TextField(
            controller: _oldCtrl,
            decoration: const InputDecoration(labelText: 'Aktuelles Passwort'),
            obscureText: true,
            enabled: !_busy,
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _newCtrl,
            decoration: const InputDecoration(labelText: 'Neues Passwort'),
            obscureText: true,
            enabled: !_busy,
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _repeatCtrl,
            decoration: const InputDecoration(labelText: 'Neues Passwort wiederholen'),
            obscureText: true,
            enabled: !_busy,
          ),
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          ],
          const SizedBox(height: 24),
          FilledButton(
            onPressed: _busy ? null : _submit,
            child: _busy
                ? const SizedBox(
                    height: 22,
                    width: 22,
                    child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                  )
                : const Text('Speichern'),
          ),
        ],
      ),
    );
  }
}
