import 'package:flutter_test/flutter_test.dart';
import 'package:schmutzfink_mobile/api/models.dart';

void main() {
  test('User.fromJson maps searcher as read-only', () {
    final user = User.fromJson({
      'id': '1',
      'username': 'such',
      'role': 'searcher',
      'must_change_password': false,
    });
    expect(user.canWrite, isFalse);
  });

  test('ApiException reads record_id on conflict', () {
    final err = ApiException.fromBody(
      409,
      '{"error":"Datei ist bereits vorhanden","record_id":"abc"}',
    );
    expect(err.duplicateId, 'abc');
    expect(err.message, contains('bereits'));
  });

  test('formatLocationLabel prefers address over coords', () {
    expect(
      formatLocationLabel(
        addressLabel: 'Vahrenwalder Str. 1, 30165 Hannover',
        lat: 52.39,
        lon: 9.73,
      ),
      'Vahrenwalder Str. 1, 30165 Hannover',
    );
    expect(
      formatLocationLabel(lat: 52.39012, lon: 9.73123),
      '52.39012, 9.73123',
    );
    expect(formatLocationLabel(), 'ohne GPS');
  });

  test('RecentUpload.fromRecordJson reads note, gps and address', () {
    final item = RecentUpload.fromRecordJson({
      'id': 'sighting-1',
      'record_id': 'photo-1',
      'note': 'Tag an der Wand',
      'uploaded_at': '2026-09-19T16:01:02Z',
      'has_gps': true,
      'lat': 52.37,
      'lon': 9.73,
      'address': {
        'label': 'Am Wall 1, 30159 Hannover',
        'street': 'Am Wall 1',
        'city': 'Hannover',
      },
    });
    expect(item.id, 'photo-1');
    expect(item.note, 'Tag an der Wand');
    expect(item.locationLabel, 'Am Wall 1, 30159 Hannover');
    expect(item.hasGps, isTrue);
  });

  test('addressLabelFromJson builds street/city fallback', () {
    expect(
      addressLabelFromJson({
        'street': 'Am Wall 1',
        'postal_code': '30159',
        'city': 'Hannover',
      }),
      'Am Wall 1, 30159 Hannover',
    );
  });
}
