import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../data/models/comic.dart';
import '../../data/providers/comic_provider.dart';
import '../../widgets/animations.dart';

/// 阅读统计页面 — 精致的数据可视化
class StatsScreen extends ConsumerWidget {
  const StatsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final statsAsync = ref.watch(statsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('阅读统计')),
      body: statsAsync.when(
        loading: () => const Center(
          child: SizedBox(
            width: 28,
            height: 28,
            child: CircularProgressIndicator(strokeWidth: 2.5),
          ),
        ),
        error: (e, _) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.error_outline_rounded,
                  size: 48, color: Theme.of(context).colorScheme.error.withOpacity(0.5)),
              const SizedBox(height: 16),
              Text('加载失败', style: TextStyle(color: Theme.of(context).colorScheme.onSurfaceVariant)),
            ],
          ),
        ),
        data: (stats) => RefreshIndicator(
          onRefresh: () => ref.refresh(statsProvider.future),
          child: ListView(
            padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
            children: [
              // ─── 总览卡片 ───
              _buildOverviewCards(context, stats),
              const SizedBox(height: 28),

              // ─── 每日阅读 ───
              _SectionTitle(title: '每日阅读', icon: Icons.show_chart_rounded),
              const SizedBox(height: 14),
              _buildDailyChart(context, stats),
              const SizedBox(height: 28),

              // ─── 最近阅读记录 ───
              _SectionTitle(title: '最近阅读', icon: Icons.history_rounded),
              const SizedBox(height: 14),
              _buildRecentSessions(context, stats),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildOverviewCards(BuildContext context, ReadingStats stats) {
    final cs = Theme.of(context).colorScheme;

    return Row(
      children: [
        Expanded(
          child: _StatCard(
            icon: Icons.timer_outlined,
            label: '总阅读',
            value: _formatDuration(stats.totalReadTime),
            gradient: [cs.primary, cs.primary.withOpacity(0.7)],
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _StatCard(
            icon: Icons.auto_stories_outlined,
            label: '阅读场次',
            value: '${stats.totalSessions}',
            gradient: [cs.secondary, cs.secondary.withOpacity(0.7)],
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _StatCard(
            icon: Icons.menu_book_outlined,
            label: '已读',
            value: '${stats.totalComicsRead}',
            gradient: [cs.tertiary, cs.tertiary.withOpacity(0.7)],
          ),
        ),
      ],
    );
  }

  Widget _buildDailyChart(BuildContext context, ReadingStats stats) {
    final cs = Theme.of(context).colorScheme;

    if (stats.safeDailyStats.isEmpty) {
      return _EmptyHint(text: '暂无每日统计数据');
    }

    final recent = stats.safeDailyStats.take(14).toList().reversed.toList();
    final allDurationZero = recent.every((d) => d.duration == 0);
    final maxVal = allDurationZero
        ? recent.fold<int>(0, (max, d) => d.sessions > max ? d.sessions : max)
        : recent.fold<int>(0, (max, d) => d.duration > max ? d.duration : max);

    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: Theme.of(context).cardTheme.color,
        borderRadius: BorderRadius.circular(16),
      ),
      child: SizedBox(
        height: 140,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: recent.map((d) {
            final ratio = maxVal > 0
                ? (allDurationZero ? d.sessions / maxVal : d.duration / maxVal)
                : 0.0;
            final tooltipMsg = allDurationZero
                ? '${d.date}\n${d.sessions} 次阅读'
                : '${d.date}\n${_formatDuration(d.duration)}';
            return Expanded(
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 2),
                child: Tooltip(
                  message: tooltipMsg,
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.end,
                    children: [
                      Flexible(
                        child: AnimatedBar(
                          heightFactor: ratio.clamp(0.05, 1.0),
                          duration: Duration(milliseconds: 600 + (recent.indexOf(d) * 40)),
                          child: Container(
                            decoration: BoxDecoration(
                              gradient: LinearGradient(
                                begin: Alignment.bottomCenter,
                                end: Alignment.topCenter,
                                colors: [
                                  cs.primary,
                                  cs.primary.withOpacity(0.4),
                                ],
                              ),
                              borderRadius: const BorderRadius.vertical(
                                  top: Radius.circular(4)),
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(height: 6),
                      Text(
                        d.date.length >= 2
                            ? d.date.substring(d.date.length - 2)
                            : d.date,
                        style: TextStyle(
                          fontSize: 9,
                          color: cs.onSurfaceVariant.withOpacity(0.5),
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            );
          }).toList(),
        ),
      ),
    );
  }

  Widget _buildRecentSessions(BuildContext context, ReadingStats stats) {
    if (stats.safeRecentSessions.isEmpty) {
      return _EmptyHint(text: '暂无阅读记录');
    }

    final sessions = stats.safeRecentSessions.take(10).toList();
    return Column(
      children: sessions.map((s) {
        return _RecentSessionTile(session: s);
      }).toList(),
    );
  }

  static String _formatDuration(int seconds) {
    if (seconds < 60) return '${seconds}s';
    if (seconds < 3600) return '${seconds ~/ 60}m';
    final h = seconds ~/ 3600;
    final m = (seconds % 3600) ~/ 60;
    return '${h}h ${m}m';
  }
}

/// 区域标题
class _SectionTitle extends StatelessWidget {
  final String title;
  final IconData icon;

  const _SectionTitle({required this.title, required this.icon});

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Row(
      children: [
        Icon(icon, size: 18, color: cs.primary.withOpacity(0.7)),
        const SizedBox(width: 8),
        Text(
          title,
          style: TextStyle(
            fontSize: 17,
            fontWeight: FontWeight.w700,
            color: cs.onSurface,
            letterSpacing: -0.3,
          ),
        ),
      ],
    );
  }
}

/// 空提示
class _EmptyHint extends StatelessWidget {
  final String text;
  const _EmptyHint({required this.text});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(32),
      decoration: BoxDecoration(
        color: Theme.of(context).cardTheme.color,
        borderRadius: BorderRadius.circular(16),
      ),
      child: Center(
        child: Text(
          text,
          style: TextStyle(
            color: Theme.of(context).colorScheme.onSurfaceVariant.withOpacity(0.4),
            fontSize: 14,
          ),
        ),
      ),
    );
  }
}

/// 最近阅读记录条目
class _RecentSessionTile extends StatelessWidget {
  final RecentSession session;

  const _RecentSessionTile({required this.session});

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    String timeStr = '';
    try {
      final dt = DateTime.parse(session.startedAt).toLocal();
      final now = DateTime.now();
      final diff = now.difference(dt);
      if (diff.inMinutes < 60) {
        timeStr = '${diff.inMinutes}分钟前';
      } else if (diff.inHours < 24) {
        timeStr = '${diff.inHours}小时前';
      } else if (diff.inDays < 7) {
        timeStr = '${diff.inDays}天前';
      } else {
        timeStr =
            '${dt.month}/${dt.day} ${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
      }
    } catch (_) {
      timeStr = session.startedAt;
    }

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      decoration: BoxDecoration(
        color: Theme.of(context).cardTheme.color,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(14),
        child: InkWell(
          borderRadius: BorderRadius.circular(14),
          onTap: () => context.push('/comic/${session.comicId}'),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
            child: Row(
              children: [
                // 图标
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: cs.primaryContainer.withOpacity(0.4),
                    borderRadius: BorderRadius.circular(11),
                  ),
                  child: Icon(Icons.auto_stories_rounded,
                      color: cs.primary, size: 18),
                ),
                const SizedBox(width: 14),
                // 信息
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        session.comicTitle,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: cs.onSurface,
                        ),
                      ),
                      const SizedBox(height: 3),
                      Text(
                        '第${session.startPage + 1}-${session.endPage + 1}页 · ${_formatDuration(session.duration)}',
                        style: TextStyle(
                          fontSize: 12,
                          color: cs.onSurfaceVariant.withOpacity(0.6),
                        ),
                      ),
                    ],
                  ),
                ),
                // 时间
                Text(
                  timeStr,
                  style: TextStyle(
                    fontSize: 11,
                    color: cs.onSurfaceVariant.withOpacity(0.4),
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  static String _formatDuration(int seconds) {
    if (seconds <= 0) return '未记录';
    if (seconds < 60) return '${seconds}秒';
    if (seconds < 3600) return '${seconds ~/ 60}分钟';
    final h = seconds ~/ 3600;
    final m = (seconds % 3600) ~/ 60;
    return '${h}小时${m}分钟';
  }
}

/// 统计卡片 — 渐变背景
class _StatCard extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;
  final List<Color> gradient;

  const _StatCard({
    required this.icon,
    required this.label,
    required this.value,
    required this.gradient,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 20, horizontal: 12),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: gradient,
        ),
        borderRadius: BorderRadius.circular(16),
        boxShadow: [
          BoxShadow(
            color: gradient.first.withOpacity(0.2),
            blurRadius: 12,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      child: Column(
        children: [
          Icon(icon, color: Colors.white.withOpacity(0.9), size: 24),
          const SizedBox(height: 10),
          AnimatedCount(
            value: int.tryParse(value.replaceAll(RegExp(r'[^0-9]'), '')) ?? 0,
            formatter: (_) => value,
            style: const TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w800,
              color: Colors.white,
              letterSpacing: -0.5,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              color: Colors.white.withOpacity(0.75),
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }
}
