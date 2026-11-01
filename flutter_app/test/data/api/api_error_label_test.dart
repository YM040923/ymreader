import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/api/api_client.dart';

DioException _status(int code, {Object? data}) {
  return DioException(
    type: DioExceptionType.badResponse,
    response: Response<dynamic>(
      statusCode: code,
      data: data,
      requestOptions: RequestOptions(path: '/api/works/work-1'),
    ),
    requestOptions: RequestOptions(path: '/api/works/work-1'),
  );
}

void main() {
  group('apiErrorLabel', () {
    test('maps transport statuses to one short sentence', () {
      expect(apiErrorLabel(_status(401)), '登录已失效，请重新登录');
      expect(apiErrorLabel(_status(403)), '没有权限执行该操作');
      expect(apiErrorLabel(_status(404)), '作品已不存在，可能刚被合并或移走');
      expect(apiErrorLabel(_status(503)), '服务器正忙，请稍后重试');
      expect(apiErrorLabel(_status(500)), '网络异常，请稍后重试');
    });

    test('surfaces the server rejection reason for a 400', () {
      final error = _status(
        400,
        data: {'error': '同一压缩包内的单元不能只拆出一部分'},
      );
      expect(apiErrorLabel(error), '同一压缩包内的单元不能只拆出一部分');
    });

    test('never leaks an HTML error page into the label', () {
      final error = _status(400, data: {'error': '<html><body>502</body></html>'});
      expect(apiErrorLabel(error), '请求无效，请检查选择后重试');
    });

    test('keeps labels free of the DioException boilerplate', () {
      for (final code in [400, 401, 403, 404, 503]) {
        final label = apiErrorLabel(_status(code));
        expect(label, isNot(contains('DioException')));
        expect(label, isNot(contains('mozilla.org')));
        expect(label.length, lessThan(40));
      }
    });

    test('falls back for non-transport errors', () {
      expect(apiErrorLabel(StateError('boom')), '操作失败，请稍后重试');
    });
  });
}
