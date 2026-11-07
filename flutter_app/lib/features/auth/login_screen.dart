import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../data/providers/auth_provider.dart';
import '../../data/providers/server_branding_provider.dart';
import '../../widgets/animations.dart';
import '../../widgets/server_logo.dart';

/// 登录 & 注册页面 — 极简优雅风格
class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen>
    with SingleTickerProviderStateMixin {
  final _usernameCtrl = TextEditingController();
  final _passwordCtrl = TextEditingController();
  final _nicknameCtrl = TextEditingController();
  bool _isRegister = false;
  bool _obscurePassword = true;

  AnimationController? _bgAnimCtrl;
  Animation<double>? _bgAnimation;

  @override
  void initState() {
    super.initState();
    _bgAnimCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    )..forward();
    _bgAnimation = CurvedAnimation(parent: _bgAnimCtrl!, curve: Curves.easeOut);

    WidgetsBinding.instance.addPostFrameCallback((_) {
      final state = ref.read(authProvider);
      if (state.needsSetup) {
        setState(() => _isRegister = true);
      }
    });
  }

  @override
  void dispose() {
    _bgAnimCtrl?.dispose();
    _usernameCtrl.dispose();
    _passwordCtrl.dispose();
    _nicknameCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final username = _usernameCtrl.text.trim();
    final password = _passwordCtrl.text.trim();
    final nickname = _nicknameCtrl.text.trim();

    if (username.isEmpty || password.isEmpty) return;

    HapticFeedback.lightImpact();
    ref.read(authProvider.notifier).clearError();

    bool ok;
    if (_isRegister) {
      ok = await ref
          .read(authProvider.notifier)
          .register(username, password, nickname);
    } else {
      ok = await ref.read(authProvider.notifier).login(username, password);
    }

    if (ok && mounted) {
      context.go('/');
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final cs = Theme.of(context).colorScheme;

    return Scaffold(
      body: _bgAnimation == null
          ? const SizedBox.shrink()
          : AnimatedBuilder(
              animation: _bgAnimation!,
              builder: (context, child) => ColoredBox(
                color: cs.surface,
                child: child,
              ),
              child: SafeArea(
                child: Center(
                  child: SingleChildScrollView(
                    padding: const EdgeInsets.symmetric(horizontal: 28),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const SizedBox(height: 48),

                        // ═══════════════════════════════════
                        // ─── Logo + 标题 ───
                        // ═══════════════════════════════════
                        _buildLogoSection(cs, authState),
                        const SizedBox(height: 44),

                        // ═══════════════════════════════════
                        // ─── 表单卡片 ───
                        // ═══════════════════════════════════
                        _buildFormCard(cs, authState),

                        // ─── 错误提示 ───
                        if (authState.error != null) ...[
                          const SizedBox(height: 12),
                          _buildErrorBanner(cs, authState.error!),
                        ],

                        const SizedBox(height: 24),

                        // ═══════════════════════════════════
                        // ─── 提交按钮 ───
                        // ═══════════════════════════════════
                        _buildSubmitButton(cs, authState),
                        const SizedBox(height: 20),

                        // ─── 切换登录/注册 ───
                        if (!authState.needsSetup) _buildToggleLink(cs),

                        const SizedBox(height: 12),

                        // ─── 切换服务器 ───
                        _buildServerLink(cs),

                        const SizedBox(height: 48),
                      ],
                    ),
                  ),
                ),
              ),
            ),
    );
  }

  // ─── Logo 区域 ───
  Widget _buildLogoSection(ColorScheme cs, AuthState authState) {
    final branding = ref.watch(currentServerBrandingProvider);
    return SlideAndFade(
      duration: const Duration(milliseconds: 600),
      child: Column(
        children: [
          Container(
            width: 72,
            height: 72,
            decoration: BoxDecoration(
              color: cs.primary,
              borderRadius: BorderRadius.circular(16),
            ),
            child: ServerLogo(branding: branding, size: 72),
          ),
          const SizedBox(height: 24),
          Text(
            branding.name,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 22,
              fontWeight: FontWeight.w700,
              color: cs.onSurface,
            ),
          ),
          const SizedBox(height: 10),
          Text(
            authState.needsSetup ? '创建管理员' : (_isRegister ? '创建账户' : '欢迎回来'),
            style: TextStyle(
              fontSize: 17,
              fontWeight: FontWeight.w600,
              color: cs.onSurface,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            authState.needsSetup
                ? '首次使用，请创建管理员账户'
                : (_isRegister ? '注册一个新账户开始阅读' : '登录以继续你的阅读之旅'),
            style: TextStyle(
              fontSize: 12,
              color: cs.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  // ─── 表单卡片 ───
  Widget _buildFormCard(ColorScheme cs, AuthState authState) {
    return SlideAndFade(
      delay: const Duration(milliseconds: 150),
      duration: const Duration(milliseconds: 500),
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: cs.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(16),
        ),
        child: Column(
          children: [
            // 用户名
            _LoginTextField(
              controller: _usernameCtrl,
              label: '用户名',
              hint: '请输入用户名',
              icon: Icons.person_outline_rounded,
              textInputAction: TextInputAction.next,
            ),

            // 昵称（注册时显示）
            AnimatedSize(
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeInOut,
              child: _isRegister
                  ? Padding(
                      padding: const EdgeInsets.only(top: 14),
                      child: _LoginTextField(
                        controller: _nicknameCtrl,
                        label: '昵称（可选）',
                        hint: '给自己取个名字',
                        icon: Icons.badge_outlined,
                        textInputAction: TextInputAction.next,
                      ),
                    )
                  : const SizedBox.shrink(),
            ),

            const SizedBox(height: 14),

            // 密码
            _LoginTextField(
              controller: _passwordCtrl,
              label: '密码',
              hint: '请输入密码',
              icon: Icons.lock_outline_rounded,
              obscureText: _obscurePassword,
              onSubmitted: (_) => _submit(),
              suffixIcon: GestureDetector(
                onTap: () =>
                    setState(() => _obscurePassword = !_obscurePassword),
                child: Padding(
                  padding: const EdgeInsets.only(right: 12),
                  child: Icon(
                    _obscurePassword
                        ? Icons.visibility_off_outlined
                        : Icons.visibility_outlined,
                    size: 20,
                    color: cs.onSurfaceVariant.withOpacity(0.35),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ─── 错误提示 ───
  Widget _buildErrorBanner(ColorScheme cs, String error) {
    return SlideAndFade(
      duration: const Duration(milliseconds: 300),
      beginOffset: const Offset(0, -0.1),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          color: cs.errorContainer.withOpacity(0.15),
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: cs.error.withOpacity(0.15)),
        ),
        child: Row(
          children: [
            Container(
              width: 28,
              height: 28,
              decoration: BoxDecoration(
                color: cs.error.withOpacity(0.1),
                borderRadius: BorderRadius.circular(8),
              ),
              child:
                  Icon(Icons.warning_amber_rounded, size: 16, color: cs.error),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                error,
                style: TextStyle(color: cs.error, fontSize: 13, height: 1.4),
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ─── 提交按钮 ───
  Widget _buildSubmitButton(ColorScheme cs, AuthState authState) {
    return SlideAndFade(
      delay: const Duration(milliseconds: 300),
      duration: const Duration(milliseconds: 500),
      child: PressableScale(
        onTap: authState.isLoading ? null : _submit,
        scaleDown: 0.97,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          width: double.infinity,
          height: 48,
          decoration: BoxDecoration(
            color: authState.isLoading
                ? cs.onSurface.withValues(alpha: 0.62)
                : cs.onSurface,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Center(
            child: authState.isLoading
                ? Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: cs.surface,
                        ),
                      ),
                      const SizedBox(width: 12),
                      Text(
                        '请稍候…',
                        style: TextStyle(
                          color: cs.surface,
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  )
                : Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        _isRegister
                            ? Icons.person_add_rounded
                            : Icons.login_rounded,
                        color: cs.surface,
                        size: 18,
                      ),
                      const SizedBox(width: 8),
                      Text(
                        _isRegister ? '注册' : '登录',
                        style: TextStyle(
                          color: cs.surface,
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
          ),
        ),
      ),
    );
  }

  // ─── 切换登录/注册 ───
  Widget _buildToggleLink(ColorScheme cs) {
    return SlideAndFade(
      delay: const Duration(milliseconds: 400),
      duration: const Duration(milliseconds: 400),
      child: GestureDetector(
        onTap: () {
          HapticFeedback.selectionClick();
          setState(() {
            _isRegister = !_isRegister;
            ref.read(authProvider.notifier).clearError();
          });
        },
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Text.rich(
            TextSpan(
              text: _isRegister ? '已有账户？ ' : '没有账户？ ',
              style: TextStyle(
                color: cs.onSurfaceVariant.withOpacity(0.5),
                fontSize: 14,
              ),
              children: [
                TextSpan(
                  text: _isRegister ? '去登录' : '去注册',
                  style: TextStyle(
                    color: cs.primary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  // ─── 切换服务器 ───
  Widget _buildServerLink(ColorScheme cs) {
    return SlideAndFade(
      delay: const Duration(milliseconds: 450),
      duration: const Duration(milliseconds: 400),
      child: GestureDetector(
        onTap: () => context.go('/server'),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.swap_horiz_rounded,
                size: 16,
                color: cs.onSurfaceVariant.withOpacity(0.35),
              ),
              const SizedBox(width: 6),
              Text(
                '切换服务器',
                style: TextStyle(
                  color: cs.onSurfaceVariant.withOpacity(0.4),
                  fontSize: 13,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════
// ─── 登录表单输入框 ───
// ═══════════════════════════════════════════════
class _LoginTextField extends StatelessWidget {
  final TextEditingController controller;
  final String label;
  final String hint;
  final IconData icon;
  final bool obscureText;
  final TextInputAction? textInputAction;
  final ValueChanged<String>? onSubmitted;
  final Widget? suffixIcon;

  const _LoginTextField({
    required this.controller,
    required this.label,
    required this.hint,
    required this.icon,
    this.obscureText = false,
    this.textInputAction,
    this.onSubmitted,
    this.suffixIcon,
  });

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(left: 2, bottom: 6),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: cs.onSurfaceVariant,
            ),
          ),
        ),
        TextField(
          controller: controller,
          obscureText: obscureText,
          textInputAction: textInputAction,
          onSubmitted: onSubmitted,
          style: TextStyle(fontSize: 14, color: cs.onSurface),
          decoration: InputDecoration(
            filled: true,
            fillColor: cs.surface,
            hintText: hint,
            hintStyle: TextStyle(fontSize: 14, color: cs.tertiary),
            prefixIcon: Icon(icon, size: 18, color: cs.onSurfaceVariant),
            suffixIcon: suffixIcon,
            isDense: true,
            contentPadding: const EdgeInsets.symmetric(vertical: 12),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: BorderSide(color: cs.outline),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: BorderSide(color: cs.onSurface, width: 1),
            ),
          ),
        ),
      ],
    );
  }
}
