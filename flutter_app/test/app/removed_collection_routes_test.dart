import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('removed collection UI and routes do not return', () {
    final router = File('lib/app/router.dart').readAsStringSync();
    final settings =
        File('lib/features/settings/settings_screen.dart').readAsStringSync();
    final source = '$router\n$settings';

    for (final removed in [
      '/collections',
      '/group/:id',
      '合集管理',
      'CollectionsScreen',
      'GroupDetailV2Screen',
    ]) {
      expect(source, isNot(contains(removed)), reason: 'found $removed');
    }
  });
}
