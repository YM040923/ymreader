Map<String, dynamic> buildScanRulesPayload({
  required bool enabled,
  required String applyOn,
  required bool aiEnabled,
  required String minConfidence,
  required bool overwriteTitle,
  required bool directoryOrganizeEnabled,
  required String directoryOrganizeMode,
  required String directoryOrganizeStrategy,
  required String hardlinkTargetDir,
}) {
  const applyOnValues = {'newOnly', 'all', 'manual'};
  const confidenceValues = {'low', 'medium', 'high'};
  const modeValues = {'hardlink', 'move'};
  const strategyValues = {'smartDir', 'flat'};

  return {
    'enabled': enabled,
    'applyOn': applyOnValues.contains(applyOn) ? applyOn : 'newOnly',
    'concurrency': 2,
    'aiInfer': {
      'enabled': aiEnabled,
      'scope': 'folderGroup',
      'minConfidence':
          confidenceValues.contains(minConfidence) ? minConfidence : 'medium',
      // Kept for backward-compatible decoding. Work metadata is the only
      // write target in the current server implementation.
      'applyToComic': false,
      'overwriteTitle': overwriteTitle,
      'fallbackToRule': true,
    },
    'directoryOrganize': {
      'enabled': directoryOrganizeEnabled,
      'mode': modeValues.contains(directoryOrganizeMode)
          ? directoryOrganizeMode
          : 'hardlink',
      'strategy': strategyValues.contains(directoryOrganizeStrategy)
          ? directoryOrganizeStrategy
          : 'smartDir',
      'hardlinkTargetDir': hardlinkTargetDir.trim(),
    },
    'filters': <String, dynamic>{},
  };
}
