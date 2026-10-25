import 'package:flutter_test/flutter_test.dart';
import 'package:schmutzfink_mobile/api/models.dart';
import 'package:schmutzfink_mobile/nearby.dart';

void main() {
  test('nearbyRadiusMeters widens with poor GPS and stays bounded', () {
    expect(nearbyRadiusMeters(null), 50);
    expect(nearbyRadiusMeters(8), 50);
    expect(nearbyRadiusMeters(40), 65);
    expect(nearbyRadiusMeters(200), 150);
  });

  test('NearbyPlace.fromJson reads motif and follow-up', () {
    final place = NearbyPlace.fromJson({
      'key': 'motif:m1',
      'motif_id': 'm1',
      'title': 'Drache',
      'case_id': 'c1',
      'last_at': '2026-01-01T00:00:00Z',
      'distance_m': 12.4,
      'count': 2,
      'sighting_id': 's1',
      'thumb_url': '/api/sightings/s1/thumb',
      'follow_up': true,
    });
    expect(place.motifId, 'm1');
    expect(place.followUp, isTrue);
    expect(place.distanceM, 12.4);
    expect(place.count, 2);
  });
}
