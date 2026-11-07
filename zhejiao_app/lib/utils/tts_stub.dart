/// 非 Web 平台的 TTS 空实现
/// Windows/macOS/Linux/Android/iOS 暂不支持 TTS
class PlatformTts {
  Future<void> speak(String text) async {}
  Future<void> pause() async {}
  Future<void> resume() async {}
  Future<void> stop() async {}
  Future<void> setLanguage(String lang) async {}
  Future<void> setSpeechRate(double rate) async {}
  Future<void> setPitch(double pitch) async {}
  void setCompletionHandler(void Function() handler) {}
  void setErrorHandler(void Function() handler) {}
}
