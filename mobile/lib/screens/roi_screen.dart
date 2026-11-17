import 'dart:io';
import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:schmutzfink_mobile/api/models.dart';

/// Ergebnis des Ausschnitt-Editors. `roi == null` heißt ganzes Bild.
class RoiPick {
  const RoiPick(this.roi);

  final Roi? roi;
}

class RoiScreen extends StatefulWidget {
  const RoiScreen({super.key, required this.file, this.initial});

  final File file;
  final Roi? initial;

  @override
  State<RoiScreen> createState() => _RoiScreenState();
}

class _RoiScreenState extends State<RoiScreen> {
  ui.Image? _image;
  Roi? _roi;
  Offset? _start;
  String? _error;

  @override
  void initState() {
    super.initState();
    _roi = widget.initial;
    _load();
  }

  @override
  void dispose() {
    _image?.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final bytes = await widget.file.readAsBytes();
      final codec = await ui.instantiateImageCodec(bytes);
      final frame = await codec.getNextFrame();
      if (!mounted) {
        frame.image.dispose();
        return;
      }
      setState(() => _image = frame.image);
    } catch (_) {
      if (mounted) setState(() => _error = 'Bild konnte nicht gelesen werden.');
    }
  }

  @override
  Widget build(BuildContext context) {
    final roi = _roi;
    return Scaffold(
      appBar: AppBar(title: const Text('Ausschnitt')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              'Das ganze Foto bleibt gespeichert. Der Ausschnitt gilt für die Ähnlichkeitssuche. Ziehen Sie ein Rechteck über das Graffiti.',
              style: Theme.of(context).textTheme.bodySmall,
            ),
            const SizedBox(height: 12),
            Expanded(child: _canvas()),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ],
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: () => Navigator.pop(context, const RoiPick(null)),
                    child: const Text('Ganzes Bild'),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: FilledButton(
                    onPressed: roi == null ? null : () => Navigator.pop(context, RoiPick(roi)),
                    child: const Text('Übernehmen'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _canvas() {
    final image = _image;
    if (image == null) {
      return const Center(child: CircularProgressIndicator());
    }
    return LayoutBuilder(
      builder: (context, constraints) {
        final view = Size(constraints.maxWidth, constraints.maxHeight);
        final fitted = _fitContain(view, image.width.toDouble(), image.height.toDouble());
        return GestureDetector(
          onPanStart: (d) {
            final p = _toNorm(d.localPosition, fitted);
            if (p == null) return;
            setState(() {
              _start = p;
              _roi = null;
              _error = null;
            });
          },
          onPanUpdate: (d) {
            final start = _start;
            final p = _toNorm(d.localPosition, fitted);
            if (start == null || p == null) return;
            setState(() => _roi = _fromCorners(start, p));
          },
          onPanEnd: (_) {
            final drawn = _roi;
            if (drawn != null && (drawn.w < 0.02 || drawn.h < 0.02)) {
              setState(() {
                _roi = widget.initial;
                _error = 'Ausschnitt zu klein.';
              });
            }
            _start = null;
          },
          child: CustomPaint(
            size: view,
            painter: _RoiPainter(image: image, fitted: fitted, roi: _roi),
          ),
        );
      },
    );
  }
}

Rect _fitContain(Size view, double imageW, double imageH) {
  final scale = math.min(view.width / imageW, view.height / imageH);
  final w = imageW * scale;
  final h = imageH * scale;
  return Rect.fromLTWH((view.width - w) / 2, (view.height - h) / 2, w, h);
}

Offset? _toNorm(Offset local, Rect fitted) {
  if (!fitted.inflate(24).contains(local)) return null;
  final x = ((local.dx - fitted.left) / fitted.width).clamp(0.0, 1.0);
  final y = ((local.dy - fitted.top) / fitted.height).clamp(0.0, 1.0);
  return Offset(x, y);
}

Roi _fromCorners(Offset a, Offset b) {
  final x = math.min(a.dx, b.dx);
  final y = math.min(a.dy, b.dy);
  return Roi(x: x, y: y, w: (a.dx - b.dx).abs(), h: (a.dy - b.dy).abs());
}

class _RoiPainter extends CustomPainter {
  _RoiPainter({required this.image, required this.fitted, required this.roi});

  final ui.Image image;
  final Rect fitted;
  final Roi? roi;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.drawImageRect(
      image,
      Rect.fromLTWH(0, 0, image.width.toDouble(), image.height.toDouble()),
      fitted,
      Paint(),
    );
    final box = roi;
    if (box == null) return;
    final rect = Rect.fromLTWH(
      fitted.left + box.x * fitted.width,
      fitted.top + box.y * fitted.height,
      box.w * fitted.width,
      box.h * fitted.height,
    );
    canvas.drawRect(
      rect,
      Paint()
        ..color = const Color(0x551F4D3A)
        ..style = PaintingStyle.fill,
    );
    canvas.drawRect(
      rect,
      Paint()
        ..color = const Color(0xFF1F4D3A)
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2,
    );
  }

  @override
  bool shouldRepaint(covariant _RoiPainter oldDelegate) {
    final a = oldDelegate.roi;
    final b = roi;
    final same = a == null
        ? b == null
        : b != null && a.x == b.x && a.y == b.y && a.w == b.w && a.h == b.h;
    return !same || oldDelegate.fitted != fitted || oldDelegate.image != image;
  }
}
