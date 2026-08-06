import 'package:flutter/foundation.dart' show kIsWeb;
import 'tts_web.dart' if (dart.library.io) 'tts_stub.dart' as platform_tts;

/// 跨平台 TTS 服务
/// Web: 使用浏览器 speechSynthesis API
/// 非 Web: 空实现（不支持 TTS）
class TtsService {
  final platform_tts.PlatformTts _impl = platform_tts.PlatformTts();

  bool get isSupported => kIsWeb;

  /// 朗读文本
  Future<void> speak(String text) => _impl.speak(text);

  /// 暂停
  Future<void> pause() => _impl.pause();

  /// 继续
  Future<void> resume() => _impl.resume();

  /// 停止
  Future<void> stop() => _impl.stop();

  /// 设置语言
  Future<void> setLanguage(String lang) => _impl.setLanguage(lang);

  /// 设置语速 (0.1 ~ 10.0, 默认 1.0)
  Future<void> setSpeechRate(double rate) => _impl.setSpeechRate(rate);

  /// 设置音调 (0 ~ 2.0, 默认 1.0)
  Future<void> setPitch(double pitch) => _impl.setPitch(pitch);

  /// 设置朗读完成回调
  void setCompletionHandler(void Function() handler) {
    _impl.setCompletionHandler(handler);
  }

  /// 设置错误回调
  void setErrorHandler(void Function() handler) {
    _impl.setErrorHandler(handler);
  }
}
