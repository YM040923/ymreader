import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/features/scan_rules/scan_rules_payload.dart';

void main() {
  test('builds Work-only scan-rule payload without collection fields', () {
    final payload = buildScanRulesPayload(
      enabled: false,
      applyOn: 'newOnly',
      aiEnabled: true,
      minConfidence: 'medium',
      overwriteTitle: false,
      directoryOrganizeEnabled: true,
      directoryOrganizeMode: 'hardlink',
      directoryOrganizeStrategy: 'smartDir',
      hardlinkTargetDir: '/app/_nowen_organized',
    );

    expect(payload.containsKey('organize'), isFalse);
    final ai = payload['aiInfer'] as Map<String, dynamic>;
    expect(ai.containsKey('applyToGroup'), isFalse);
    expect(ai['applyToComic'], isFalse);
    expect(ai['scope'], 'folderGroup');
    expect(payload['enabled'], isFalse);
  });

  test('normalizes invalid choices to server-supported defaults', () {
    final payload = buildScanRulesPayload(
      enabled: true,
      applyOn: 'unexpected',
      aiEnabled: false,
      minConfidence: 'unexpected',
      overwriteTitle: true,
      directoryOrganizeEnabled: true,
      directoryOrganizeMode: 'unexpected',
      directoryOrganizeStrategy: 'unexpected',
      hardlinkTargetDir: '  /organized  ',
    );
    final ai = payload['aiInfer'] as Map<String, dynamic>;
    final directory = payload['directoryOrganize'] as Map<String, dynamic>;

    expect(payload['applyOn'], 'newOnly');
    expect(ai['minConfidence'], 'medium');
    expect(directory['mode'], 'hardlink');
    expect(directory['strategy'], 'smartDir');
    expect(directory['hardlinkTargetDir'], '/organized');
  });
}
