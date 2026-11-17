import 'dart:math' as math;

/// Umkreis für „Funde in der Nähe“: mindestens 50 m, bei ungenauem GPS bis 150 m.
double nearbyRadiusMeters(double? accuracyM) {
  final acc = (accuracyM != null && accuracyM.isFinite && accuracyM > 0) ? accuracyM : 0.0;
  return math.max(50, math.min(150, acc + 25));
}
