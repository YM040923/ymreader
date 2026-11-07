import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'app_switch.dart';

const _accent = Color(0xFFFF7A6B);

/// 漫画阅读模式
enum ComicReadingMode { single, webtoon, doublePage }

/// 阅读方向
enum ReadingDirection { ltr, rtl, ttb }

/// 适应显示模式
enum FitMode { contain, width, height }

/// 屏幕方向锁定
enum OrientationLock { followSystem, portrait, landscape }

T _enumValue<T>(List<T> values, int? index, T fallback) {
  if (index == null || index < 0 || index >= values.length) return fallback;
  return values[index];
}

/// 阅读器设置 — 持久化到 SharedPreferences
class ReaderSettings {
  static const int settingsVersion = 5;

  /// 「适应显示」把旧默认的完整容纳改成适应宽度所在的版本。迁移判定必须钉在
  /// 这个版本上，否则以后每加一条迁移都会把用户后来自选的完整容纳抹掉。
  static const int _fitModeMigrationVersion = 4;

  /// 「连续阅读」默认改为开启所在的版本：旧默认关，但面板保存时会把当时看到的
  /// 值全量写盘，所以存档里的 false 多半只是默认值，跨到这个版本时统一翻开一次。
  static const int _continuousReadingMigrationVersion = 5;

  /// 亮度滑条的下限：再低基本就看不清了
  static const double minBrightness = 0.25;

  final ComicReadingMode mode;
  final ReadingDirection direction;
  final FitMode fitMode;
  final bool showPageNumber;
  final int autoPageInterval; // 秒，0=禁用
  final bool continuousReading;

  /// 双页模式下封面单独显示（错页 1 页），日漫见开页对齐用
  final bool doubleCoverAlone;

  /// 双页模式贴合（去除中间缝隙），让两页在屏幕中央对齐拼接
  final bool doublePageNoGap;

  /// 阅读亮度，1.0 = 跟随系统（不额外压暗）
  final double brightness;

  /// 屏幕方向锁定
  final OrientationLock orientationLock;

  /// 阅读时左右点击区域对调（左翻上页 → 右翻上页）
  final bool swapTapZones;

  /// 使用音量键翻页
  final bool volumeKeyPaging;

  const ReaderSettings({
    this.mode = ComicReadingMode.single,
    this.direction = ReadingDirection.ltr,
    this.fitMode = FitMode.width,
    this.showPageNumber = true,
    this.autoPageInterval = 0,
    // 跨过话边界继续滚是阅读器的默认体验，只有用户明确关掉才退回单话。
    this.continuousReading = true,
    this.doubleCoverAlone = true,
    this.doublePageNoGap = true,
    this.brightness = 1.0,
    this.orientationLock = OrientationLock.followSystem,
    this.swapTapZones = false,
    this.volumeKeyPaging = false,
  });

  /// 需要落盘的键，[clearScope] 与读写共用
  static const List<String> _keys = [
    'reader_mode',
    'reader_direction',
    'reader_fitMode',
    'reader_showPageNumber',
    'reader_autoPageInterval',
    'reader_continuousReading',
    'reader_doubleCoverAlone',
    'reader_doublePageNoGap',
    'reader_brightnessPercent',
    'reader_orientationLock',
    'reader_swapTapZones',
    'reader_volumeKeyPaging',
  ];

  ReaderSettings copyWith({
    ComicReadingMode? mode,
    ReadingDirection? direction,
    FitMode? fitMode,
    bool? showPageNumber,
    int? autoPageInterval,
    bool? continuousReading,
    bool? doubleCoverAlone,
    bool? doublePageNoGap,
    double? brightness,
    OrientationLock? orientationLock,
    bool? swapTapZones,
    bool? volumeKeyPaging,
  }) {
    return ReaderSettings(
      mode: mode ?? this.mode,
      direction: direction ?? this.direction,
      fitMode: fitMode ?? this.fitMode,
      showPageNumber: showPageNumber ?? this.showPageNumber,
      autoPageInterval: autoPageInterval ?? this.autoPageInterval,
      continuousReading: continuousReading ?? this.continuousReading,
      doubleCoverAlone: doubleCoverAlone ?? this.doubleCoverAlone,
      doublePageNoGap: doublePageNoGap ?? this.doublePageNoGap,
      brightness: brightness ?? this.brightness,
      orientationLock: orientationLock ?? this.orientationLock,
      swapTapZones: swapTapZones ?? this.swapTapZones,
      volumeKeyPaging: volumeKeyPaging ?? this.volumeKeyPaging,
    );
  }

  /// 从 SharedPreferences 读取。
  ///
  /// v2 将旧版默认的“完整容纳”迁移为“适应宽度”。旧默认会把长图完整
  /// 压进一屏，导致漫画文字非常小；用户仍可在设置中切回完整显示。
  ///
  /// v5 把“连续阅读”的默认值改为开启，并把旧存档里由默认值写下的关闭
  /// 翻开一次；此后用户自己关掉仍然生效。
  static Future<ReaderSettings> load({
    String? scope,
    String? fallbackScope,
  }) async {
    final prefs = await SharedPreferences.getInstance();
    String scoped(String key) =>
        scope == null || scope.isEmpty ? key : '${key}_$scope';
    String fallbackKey(String key) =>
        fallbackScope == null || fallbackScope.isEmpty
            ? key
            : '${key}_$fallbackScope';
    // Work override > “全部作品” global > library default. Otherwise a global
    // change made from a book is silently undone by a library default when the
    // next chapter opens a fresh reader and re-reads the settings.
    int? readInt(String key) =>
        prefs.getInt(scoped(key)) ??
        prefs.getInt(key) ??
        prefs.getInt(fallbackKey(key));
    bool? readBool(String key) =>
        prefs.getBool(scoped(key)) ??
        prefs.getBool(key) ??
        prefs.getBool(fallbackKey(key));
    final storedVersion = prefs.getInt('reader_settings_version') ?? 1;
    var fitModeIndex = readInt('reader_fitMode');

    if (fitModeIndex == null ||
        (storedVersion < _fitModeMigrationVersion &&
            fitModeIndex == FitMode.contain.index)) {
      fitModeIndex = FitMode.width.index;
      await prefs.setInt('reader_fitMode', fitModeIndex);
    }
    if (storedVersion < _continuousReadingMigrationVersion) {
      // 三个作用域一起翻，不然作品级存档里的 false 会永远留着。
      for (final key in [
        scoped('reader_continuousReading'),
        fallbackKey('reader_continuousReading'),
        'reader_continuousReading',
      ]) {
        if (prefs.getBool(key) == false) {
          await prefs.setBool(key, true);
        }
      }
    }
    if (storedVersion < settingsVersion) {
      await prefs.setInt('reader_settings_version', settingsVersion);
    }

    // 亮度按整数百分比存，避免浮点键位差异
    final brightnessPercent = readInt('reader_brightnessPercent');

    return ReaderSettings(
      mode: _enumValue(
        ComicReadingMode.values,
        readInt('reader_mode'),
        ComicReadingMode.single,
      ),
      direction: _enumValue(
        ReadingDirection.values,
        readInt('reader_direction'),
        ReadingDirection.ltr,
      ),
      fitMode: _enumValue(
        FitMode.values,
        readInt('reader_fitMode') ?? fitModeIndex,
        FitMode.width,
      ),
      showPageNumber: readBool('reader_showPageNumber') ?? true,
      autoPageInterval: readInt('reader_autoPageInterval') ?? 0,
      // 没存过的用户拿到新默认值；存过 false 的仍然按他自己的选择。
      continuousReading: readBool('reader_continuousReading') ?? true,
      doubleCoverAlone: readBool('reader_doubleCoverAlone') ?? true,
      doublePageNoGap: readBool('reader_doublePageNoGap') ?? true,
      brightness: brightnessPercent == null
          ? 1.0
          : (brightnessPercent.clamp(25, 100) / 100.0),
      orientationLock: _enumValue(
        OrientationLock.values,
        readInt('reader_orientationLock'),
        OrientationLock.followSystem,
      ),
      swapTapZones: readBool('reader_swapTapZones') ?? false,
      volumeKeyPaging: readBool('reader_volumeKeyPaging') ?? false,
    );
  }

  /// 保存到 SharedPreferences
  Future<void> save({String? scope}) async {
    final prefs = await SharedPreferences.getInstance();
    String scoped(String key) =>
        scope == null || scope.isEmpty ? key : '${key}_$scope';
    await prefs.setInt('reader_settings_version', settingsVersion);
    await prefs.setInt(scoped('reader_mode'), mode.index);
    await prefs.setInt(scoped('reader_direction'), direction.index);
    await prefs.setInt(scoped('reader_fitMode'), fitMode.index);
    await prefs.setBool(scoped('reader_showPageNumber'), showPageNumber);
    await prefs.setInt(scoped('reader_autoPageInterval'), autoPageInterval);
    await prefs.setBool(scoped('reader_continuousReading'), continuousReading);
    await prefs.setBool(scoped('reader_doubleCoverAlone'), doubleCoverAlone);
    await prefs.setBool(scoped('reader_doublePageNoGap'), doublePageNoGap);
    await prefs.setInt(scoped('reader_brightnessPercent'),
        (brightness * 100).round().clamp(25, 100));
    await prefs.setInt(scoped('reader_orientationLock'), orientationLock.index);
    await prefs.setBool(scoped('reader_swapTapZones'), swapTapZones);
    await prefs.setBool(scoped('reader_volumeKeyPaging'), volumeKeyPaging);
  }

  static Future<void> clearScope(String scope) async {
    final prefs = await SharedPreferences.getInstance();
    for (final key in _keys) {
      await prefs.remove('${key}_$scope');
    }
  }
}

/// 阅读器设置面板（底部弹出）
class ReaderSettingsPanel extends StatefulWidget {
  final ReaderSettings settings;
  final ValueChanged<ReaderSettings> onChanged;
  final String? scope;

  const ReaderSettingsPanel({
    super.key,
    required this.settings,
    required this.onChanged,
    this.scope,
  });

  @override
  State<ReaderSettingsPanel> createState() => _ReaderSettingsPanelState();

  /// 从底部弹出显示
  static void show(BuildContext context,
      {required ReaderSettings settings,
      required ValueChanged<ReaderSettings> onChanged,
      String? scope}) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => ReaderSettingsPanel(
        settings: settings,
        onChanged: onChanged,
        scope: scope,
      ),
    );
  }
}

class _ReaderSettingsPanelState extends State<ReaderSettingsPanel> {
  late ReaderSettings _settings;

  /// 库默认页没有“对全部作品生效”这一层，直接编辑库级设置
  bool get _isLibraryScope => widget.scope?.startsWith('library:') ?? false;
  bool get _isGlobalScope => widget.scope == null || widget.scope!.isEmpty;

  /// true = 本次编辑写入全局键，且本书的独立设置已被清除
  late bool _appliesToAll = _isGlobalScope;

  @override
  void initState() {
    super.initState();
    _settings = widget.settings;
    _detectScopeMode();
  }

  /// 本书没有独立键时，说明它正在吃全局设置，开关应显示“全部作品”
  Future<void> _detectScopeMode() async {
    final scope = widget.scope;
    if (_isGlobalScope || _isLibraryScope || scope == null || scope.isEmpty) {
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    final hasOwn = prefs
        .getKeys()
        .any((key) => key.endsWith('_$scope') && key.startsWith('reader_'));
    if (!hasOwn && mounted) setState(() => _appliesToAll = true);
  }

  String? get _writeScope => _appliesToAll ? null : widget.scope;

  void _update(ReaderSettings newSettings) {
    setState(() => _settings = newSettings);
    widget.onChanged(newSettings);
    newSettings.save(scope: _writeScope);
  }

  Future<void> _setAppliesToAll(bool on) async {
    if (on) {
      await _settings.save(); // 先落全局
      await ReaderSettings.clearScope(widget.scope ?? '');
      setState(() => _appliesToAll = true);
    } else {
      setState(() => _appliesToAll = false);
      await _settings.save(scope: widget.scope);
    }
  }

  @override
  Widget build(BuildContext context) {
    final size = MediaQuery.sizeOf(context);
    // 横屏时底部弹窗只剩小半屏高，改成居中卡片更好读
    if (size.width > size.height) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxWidth: 560,
              maxHeight: size.height - 32,
            ),
            child: _card(context, null),
          ),
        ),
      );
    }
    return DraggableScrollableSheet(
      initialChildSize: 0.74,
      minChildSize: 0.34,
      maxChildSize: 0.90,
      builder: (context, scrollController) => _card(context, scrollController),
    );
  }

  Widget _card(BuildContext context, ScrollController? scrollController) {
    final isPaged = _settings.mode != ComicReadingMode.webtoon;
    final floating = scrollController == null;
    final radius = floating
        ? const BorderRadius.all(Radius.circular(16))
        : const BorderRadius.vertical(top: Radius.circular(16));
    return ClipRRect(
      borderRadius: radius,
      child: Container(
        decoration: BoxDecoration(
          color: const Color(0xFF17171A),
          borderRadius: radius,
          border: floating
              ? Border.all(color: Colors.white.withValues(alpha: 0.08))
              : Border(
                  top: BorderSide(color: Colors.white.withValues(alpha: 0.08)),
                ),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 6),
            Center(
              child: Container(
                width: 32,
                height: 4,
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.18),
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: 6),
            // 标题栏
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
              child: Row(
                children: [
                  const Text(
                    '阅读设置',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const Spacer(),
                  if (!_isLibraryScope && !_isGlobalScope)
                    _ScopeChip(appliesToAll: _appliesToAll)
                  else if (_isGlobalScope)
                    const _ScopeChip(appliesToAll: true),
                ],
              ),
            ),
            // 可滚动的设置项
            Expanded(
              child: ListView(
                controller: scrollController,
                padding: const EdgeInsets.fromLTRB(20, 6, 20, 0),
                children: [
                  // ── 画面 ──
                  const _SectionTitle('画面'),
                  const _SettingLabel('适应显示'),
                  _ToggleGroup<FitMode>(
                    value: _settings.fitMode,
                    items: const [
                      _ToggleItem(FitMode.contain, '完整'),
                      _ToggleItem(FitMode.width, '宽度'),
                      _ToggleItem(FitMode.height, '高度'),
                    ],
                    onChanged: (v) => _update(_settings.copyWith(fitMode: v)),
                  ),
                  if (_settings.mode == ComicReadingMode.webtoon &&
                      _settings.direction != ReadingDirection.ttb)
                    const _Hint('横向长条按屏幕高度铺满，适应显示只在竖向长条和翻页模式生效'),
                  const _SettingLabel('页面渲染'),
                  _ToggleGroup<ComicReadingMode>(
                    value: _settings.mode,
                    items: const [
                      _ToggleItem(ComicReadingMode.single, '单页'),
                      _ToggleItem(ComicReadingMode.doublePage, '双页'),
                      _ToggleItem(ComicReadingMode.webtoon, '长条'),
                    ],
                    onChanged: (v) => _update(_settings.copyWith(mode: v)),
                  ),

                  // 双页：封面单独显示（错页对齐）
                  if (_settings.mode == ComicReadingMode.doublePage) ...[
                    _SwitchRow(
                      label: '封面单独显示',
                      hint: '双页错页',
                      value: _settings.doubleCoverAlone,
                      onChanged: (v) =>
                          _update(_settings.copyWith(doubleCoverAlone: v)),
                    ),
                    const _Hint(
                      '关闭后从第 1 页开始两两配对，适合欧美漫画；开启后第 1 页单独显示，对齐日漫见开页',
                    ),
                    _SwitchRow(
                      label: '双页贴合',
                      hint: '去除中间缝',
                      value: _settings.doublePageNoGap,
                      onChanged: (v) =>
                          _update(_settings.copyWith(doublePageNoGap: v)),
                    ),
                    const _Hint(
                      '开启后两页在屏幕中央贴合拼接，跨页大图观感更佳；关闭则两页各自居中',
                    ),
                  ],

                  const _SettingLabel('阅读方向'),
                  _ToggleGroup<ReadingDirection>(
                    value: _settings.direction,
                    items: const [
                      _ToggleItem(ReadingDirection.ltr, '左→右'),
                      _ToggleItem(ReadingDirection.rtl, '右→左'),
                      _ToggleItem(ReadingDirection.ttb, '上→下'),
                    ],
                    onChanged: (v) {
                      var s = _settings.copyWith(direction: v);
                      // 上→下 自动切换为长条模式
                      if (v == ReadingDirection.ttb) {
                        s = s.copyWith(mode: ComicReadingMode.webtoon);
                      }
                      _update(s);
                    },
                  ),

                  // ── 屏幕 ──
                  const _SectionTitle('屏幕'),
                  _SliderRow(
                    label: '亮度',
                    icon: _settings.brightness >= 1.0
                        ? Icons.brightness_high_rounded
                        : Icons.brightness_6_rounded,
                    value: _settings.brightness,
                    min: ReaderSettings.minBrightness,
                    max: 1.0,
                    trailing: _settings.brightness >= 1.0
                        ? const _MiniTag('跟随系统')
                        : null,
                    trailingAction: _settings.brightness < 1.0
                        ? () => _update(_settings.copyWith(brightness: 1.0))
                        : null,
                    trailingActionLabel: '还原',
                    onChanged: (v) =>
                        _update(_settings.copyWith(brightness: v)),
                  ),
                  const _SettingLabel('方向锁定'),
                  _ToggleGroup<OrientationLock>(
                    value: _settings.orientationLock,
                    items: const [
                      _ToggleItem(OrientationLock.followSystem, '跟随'),
                      _ToggleItem(OrientationLock.portrait, '竖屏'),
                      _ToggleItem(OrientationLock.landscape, '横屏'),
                    ],
                    onChanged: (v) =>
                        _update(_settings.copyWith(orientationLock: v)),
                  ),

                  // ── 操作 ──
                  const _SectionTitle('操作'),
                  _SwitchRow(
                    label: '音量键翻页',
                    value: _settings.volumeKeyPaging && isPaged,
                    disabled: !isPaged,
                    onChanged: (v) =>
                        _update(_settings.copyWith(volumeKeyPaging: v)),
                  ),
                  _SwitchRow(
                    label: '左右点击区对调',
                    value: _settings.swapTapZones && isPaged,
                    disabled: !isPaged,
                    onChanged: (v) =>
                        _update(_settings.copyWith(swapTapZones: v)),
                  ),
                  _SwitchRow(
                    label: '页码指示器',
                    value: _settings.showPageNumber,
                    onChanged: (v) =>
                        _update(_settings.copyWith(showPageNumber: v)),
                  ),
                  _SwitchRow(
                    label: '连续阅读',
                    value: _settings.continuousReading,
                    onChanged: (v) =>
                        _update(_settings.copyWith(continuousReading: v)),
                  ),
                  const _Hint('开启后当前话结束会自动接上下一话'),

                  // ── 自动阅读 ──
                  const _SectionTitle('自动阅读'),
                  _ToggleGroup<int>(
                    value: _autoSpeedValue,
                    items: const [
                      _ToggleItem(0, '关'),
                      _ToggleItem(6, '慢'),
                      _ToggleItem(4, '标准'),
                      _ToggleItem(2, '快'),
                    ],
                    onChanged: (v) =>
                        _update(_settings.copyWith(autoPageInterval: v)),
                  ),
                  const _Hint('翻页模式按间隔跳下一页，长条模式匀速下滚；翻到本话最后一会自动接下一话'),

                  // ── 生效范围 ──
                  if (!_isLibraryScope) ...[
                    const SizedBox(height: 22),
                    _SwitchRow(
                      label: '对全部作品生效',
                      value: _appliesToAll,
                      onChanged: _setAppliesToAll,
                    ),
                    _Hint(_appliesToAll
                        ? '本书已改用全局设置；关掉开关可恢复单独设置本书'
                        : '打开后这套设置成为所有作品的默认值'),
                  ],
                  const SizedBox(height: 26),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// 自动阅读档位：把历史遗留的任意秒数归到最近的一档
  int get _autoSpeedValue {
    final v = _settings.autoPageInterval;
    if (v <= 0) return 0;
    const steps = [6, 4, 2];
    var best = steps.first;
    for (final s in steps) {
      if ((v - s).abs() < (v - best).abs()) best = s;
    }
    return best;
  }
}

// ── 辅助组件 ──

class _SectionTitle extends StatelessWidget {
  final String title;
  const _SectionTitle(this.title);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 20, bottom: 10),
      child: Text(
        title,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: Colors.white.withValues(alpha: 0.58),
        ),
      ),
    );
  }
}

class _SettingLabel extends StatelessWidget {
  final String text;
  const _SettingLabel(this.text);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 7),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.w500,
          color: Colors.white.withValues(alpha: 0.82),
        ),
      ),
    );
  }
}

class _Hint extends StatelessWidget {
  final String text;
  const _Hint(this.text);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 6, bottom: 2),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 11,
          height: 1.45,
          color: Colors.white.withValues(alpha: 0.46),
        ),
      ),
    );
  }
}

class _ScopeChip extends StatelessWidget {
  final bool appliesToAll;
  const _ScopeChip({required this.appliesToAll});

  @override
  Widget build(BuildContext context) {
    final color = appliesToAll ? _accent : Colors.white.withValues(alpha: 0.55);
    return Text(
      appliesToAll ? '全部作品' : '仅本书',
      style: TextStyle(fontSize: 11, color: color, height: 1.2),
    );
  }
}

class _MiniTag extends StatelessWidget {
  final String text;
  const _MiniTag(this.text);

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: TextStyle(
        fontSize: 11,
        color: Colors.white.withValues(alpha: 0.55),
      ),
    );
  }
}

class _ToggleItem<T> {
  final T value;
  final String label;
  const _ToggleItem(this.value, this.label);
}

class _ToggleGroup<T> extends StatelessWidget {
  final T value;
  final List<_ToggleItem<T>> items;
  final ValueChanged<T> onChanged;

  const _ToggleGroup({
    required this.value,
    required this.items,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: Wrap(
        spacing: 8,
        runSpacing: 8,
        children: items.map((item) {
          final selected = value == item.value;
          return GestureDetector(
            onTap: () => onChanged(item.value),
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 140),
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              decoration: BoxDecoration(
                color: selected
                    ? _accent.withValues(alpha: 0.10)
                    : Colors.white.withValues(alpha: 0.04),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color:
                      selected ? _accent : Colors.white.withValues(alpha: 0.06),
                ),
              ),
              child: Text(
                item.label,
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w500,
                  color: selected ? _accent : Colors.white70,
                ),
              ),
            ),
          );
        }).toList(),
      ),
    );
  }
}

class _SwitchRow extends StatelessWidget {
  final String label;
  final String? hint;
  final bool value;
  final bool disabled;
  final ValueChanged<bool> onChanged;

  const _SwitchRow({
    required this.label,
    required this.value,
    required this.onChanged,
    this.hint,
    this.disabled = false,
  });

  @override
  Widget build(BuildContext context) {
    // 当前模式下用不上的项不隐藏：降低文字和开关的对比度即可。
    // 之前加删除线会把中文笔画切碎，看起来像渲染坏了。
    final on = disabled ? false : value;
    final labelColor = disabled
        ? Colors.white.withValues(alpha: 0.42)
        : Colors.white.withValues(alpha: 0.90);
    return SizedBox(
      height: 52,
      // 参照做法：整行都是点击区，只留开关本身一小块是合理的
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: disabled ? null : () => onChanged(!value),
        child: Row(
          children: [
            Text(
              label,
              style: TextStyle(fontSize: 14, color: labelColor),
            ),
            if (hint != null && !disabled) ...[
              const SizedBox(width: 6),
              Text(
                hint!,
                style: TextStyle(
                  fontSize: 11,
                  color: Colors.white.withValues(alpha: 0.46),
                ),
              ),
            ],
            const Spacer(),
            AppSwitch(
              value: on,
              enabled: !disabled,
              activeColor: _accent,
              inactiveColor: Colors.white.withValues(alpha: 0.14),
            ),
          ],
        ),
      ),
    );
  }
}

class _SliderRow extends StatelessWidget {
  final String label;
  final IconData icon;
  final double value;
  final double min;
  final double max;
  final Widget? trailing;
  final VoidCallback? trailingAction;
  final String? trailingActionLabel;
  final ValueChanged<double> onChanged;

  const _SliderRow({
    required this.label,
    required this.icon,
    required this.value,
    required this.min,
    required this.max,
    required this.onChanged,
    this.trailing,
    this.trailingAction,
    this.trailingActionLabel,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          Icon(icon, size: 17, color: Colors.white.withValues(alpha: 0.55)),
          const SizedBox(width: 8),
          SizedBox(
            width: 34,
            child: Text(
              label,
              style: TextStyle(
                fontSize: 13,
                color: Colors.white.withValues(alpha: 0.78),
              ),
            ),
          ),
          Expanded(
            child: SliderTheme(
              data: SliderTheme.of(context).copyWith(
                trackHeight: 2,
                activeTrackColor: _accent,
                inactiveTrackColor: Colors.white.withValues(alpha: 0.18),
                thumbColor: _accent,
                overlayColor: _accent.withValues(alpha: 0.14),
                thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 6),
                overlayShape: const RoundSliderOverlayShape(overlayRadius: 16),
              ),
              child: Slider(
                value: value.clamp(min, max),
                min: min,
                max: max,
                onChanged: onChanged,
              ),
            ),
          ),
          SizedBox(
            width: 62,
            child: Align(
              alignment: Alignment.centerRight,
              child: trailingAction != null
                  ? TextButton(
                      onPressed: trailingAction,
                      style: TextButton.styleFrom(
                        padding: const EdgeInsets.symmetric(horizontal: 6),
                        minimumSize: Size.zero,
                        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                        foregroundColor: _accent,
                      ),
                      child: Text(
                        trailingActionLabel ?? '',
                        style: const TextStyle(fontSize: 11),
                      ),
                    )
                  : (trailing ?? const SizedBox.shrink()),
            ),
          ),
        ],
      ),
    );
  }
}
