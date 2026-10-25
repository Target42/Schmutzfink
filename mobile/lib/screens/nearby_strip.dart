import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:schmutzfink_mobile/api/models.dart';

class NearbyPanel extends StatelessWidget {
  const NearbyPanel({
    super.key,
    required this.places,
    required this.radiusM,
    required this.selectedKey,
    required this.onSelect,
    required this.loadThumb,
  });

  final List<NearbyPlace> places;
  final double radiusM;
  final String? selectedKey;
  final ValueChanged<NearbyPlace> onSelect;
  final Future<Uint8List> Function(String url) loadThumb;

  @override
  Widget build(BuildContext context) {
    if (places.isEmpty) return const SizedBox.shrink();
    NearbyPlace? prompt;
    for (final place in places) {
      if (place.followUp && place.key != selectedKey) {
        prompt = place;
        break;
      }
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('In der Nähe', style: Theme.of(context).textTheme.titleMedium),
        Text(
          'im Umkreis von ${radiusM.round()} m — Tippen übernimmt Motiv und offenen Vorgang',
          style: Theme.of(context).textTheme.bodySmall,
        ),
        if (prompt != null) ...[
          const SizedBox(height: 8),
          Material(
            color: Theme.of(context).colorScheme.secondaryContainer,
            borderRadius: BorderRadius.circular(12),
            child: InkWell(
              borderRadius: BorderRadius.circular(12),
              onTap: () => onSelect(prompt!),
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Text(
                  '„${prompt.title}“ zuletzt am ${_fmtDate(prompt.lastAt)} — Kontrollfoto?',
                ),
              ),
            ),
          ),
        ],
        const SizedBox(height: 8),
        SizedBox(
          height: 108,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            itemCount: places.length,
            separatorBuilder: (_, _) => const SizedBox(width: 8),
            itemBuilder: (context, i) {
              final place = places[i];
              final selected = place.key == selectedKey;
              return _PlaceCard(
                place: place,
                selected: selected,
                loadThumb: loadThumb,
                onTap: () => onSelect(place),
              );
            },
          ),
        ),
      ],
    );
  }
}

class _PlaceCard extends StatelessWidget {
  const _PlaceCard({
    required this.place,
    required this.selected,
    required this.loadThumb,
    required this.onTap,
  });

  final NearbyPlace place;
  final bool selected;
  final Future<Uint8List> Function(String url) loadThumb;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Material(
      color: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: selected ? scheme.primary : Colors.black26, width: selected ? 2 : 1),
      ),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: onTap,
        child: SizedBox(
          width: 220,
          child: Padding(
            padding: const EdgeInsets.all(8),
            child: Row(
              children: [
                _Thumb(url: place.thumbUrl, load: loadThumb),
                const SizedBox(width: 8),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Text(place.title, maxLines: 2, overflow: TextOverflow.ellipsis),
                      const SizedBox(height: 4),
                      Text(
                        '${place.distanceM.round()} m · ${_fmtDate(place.lastAt)}',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                      if (place.followUp)
                        Text('Kontrollfoto', style: Theme.of(context).textTheme.labelSmall),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _Thumb extends StatefulWidget {
  const _Thumb({required this.url, required this.load});

  final String url;
  final Future<Uint8List> Function(String url) load;

  @override
  State<_Thumb> createState() => _ThumbState();
}

class _ThumbState extends State<_Thumb> {
  Uint8List? _bytes;

  @override
  void initState() {
    super.initState();
    _fetch();
  }

  @override
  void didUpdateWidget(covariant _Thumb oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.url != widget.url) {
      _bytes = null;
      _fetch();
    }
  }

  Future<void> _fetch() async {
    if (widget.url.isEmpty) return;
    try {
      final bytes = await widget.load(widget.url);
      if (!mounted) return;
      setState(() => _bytes = bytes);
    } catch (_) {}
  }

  @override
  Widget build(BuildContext context) {
    final bytes = _bytes;
    return ClipRRect(
      borderRadius: BorderRadius.circular(8),
      child: SizedBox(
        width: 72,
        height: 72,
        child: bytes == null
            ? const ColoredBox(color: Colors.black12, child: Icon(Icons.image_outlined))
            : Image.memory(bytes, fit: BoxFit.cover),
      ),
    );
  }
}

String _fmtDate(DateTime t) {
  final local = t.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(local.day)}.${two(local.month)}.${local.year}';
}
