// ignore: avoid_web_libraries_in_flutter
import 'dart:async';
import 'dart:html' as html;

/// Web 平台 TTS 实现
/// 使用浏览器原生 speechSynthesis API
class PlatformTts {
  html.SpeechSynthesisUtterance? _utterance;
  String _language = 'zh-CN';
  double _rate = 1.0;
  double _pitch = 1.0;
  void Function()? _onComplete;
  void Function()? _onError;

  // 维护事件订阅引用，防止重复注册
  StreamSubscription? _endSubscription;
  StreamSubscription? _errorSubscription;

  Future<void> speak(String text) async {
    // 先取消之前的朗读和事件订阅
    html.window.speechSynthesis!.cancel();
    _endSubscription?.cancel();
    _errorSubscription?.cancel();

    if (text.trim().isEmpty) return;

    _utterance = html.SpeechSynthesisUtterance(text);
    _utterance!.lang = _language;
    _utterance!.rate = _rate;
    _utterance!.pitch = _pitch;

    // 尝试选择中文语音
    final voices = html.window.speechSynthesis!.getVoices();
    html.SpeechSynthesisVoice? zhVoice;
    for (final voice in voices) {
      if (voice.lang?.startsWith('zh') == true) {
        zhVoice = voice;
        break;
      }
    }
    zhVoice ??= voices
        .where((v) => v.lang?.contains('CN') == true)
        .cast<html.SpeechSynthesisVoice?>()
        .firstWhere((_) => true, orElse: () => null);
    if (zhVoice != null) {
      _utterance!.voice = zhVoice;
    }

    // 设置回调（维护订阅引用，下次 speak 前取消）
    _endSubscription = _utterance!.onEnd.listen((_) {
      _onComplete?.call();
    });

    _errorSubscription = _utterance!.onError.listen((_) {
      _onError?.call();
    });

    html.window.speechSynthesis!.speak(_utterance!);
  }

  Future<void> pause() async {
    html.window.speechSynthesis!.pause();
  }

  Future<void> resume() async {
    html.window.speechSynthesis!.resume();
  }

  Future<void> stop() async {
    html.window.speechSynthesis!.cancel();
  }

  Future<void> setLanguage(String lang) async {
    _language = lang;
  }

  Future<void> setSpeechRate(double rate) async {
    _rate = rate;
  }

  Future<void> setPitch(double pitch) async {
    _pitch = pitch;
  }

  void setCompletionHandler(void Function() handler) {
    _onComplete = handler;
  }

  void setErrorHandler(void Function() handler) {
    _onError = handler;
  }
}
