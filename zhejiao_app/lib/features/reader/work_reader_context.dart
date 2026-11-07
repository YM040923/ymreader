import '../../data/models/work.dart';

class WorkReaderContext {
  final Work work;
  final String unitId;

  const WorkReaderContext({required this.work, required this.unitId});

  WorkUnit get currentUnit => work.units.firstWhere(
        (unit) => unit.id == unitId,
        orElse: () => work.units.first,
      );

  int get currentIndex =>
      work.units.indexWhere((unit) => unit.id == currentUnit.id);

  WorkUnit? get previousUnit =>
      currentIndex > 0 ? work.units[currentIndex - 1] : null;

  WorkUnit? get nextUnit =>
      currentIndex >= 0 && currentIndex + 1 < work.units.length
          ? work.units[currentIndex + 1]
          : null;

  int toRelativePage(int absolutePage) => (absolutePage - currentUnit.startPage)
      .clamp(0, currentUnit.pageCount - 1);

  int toAbsolutePage(int relativePage) =>
      currentUnit.startPage + relativePage.clamp(0, currentUnit.pageCount - 1);

  String routeFor(WorkUnit unit, {int? absolutePage}) {
    final page = absolutePage ?? unit.startPage;
    final query = Uri(queryParameters: {
      'page': page.toString(),
      'workId': work.id,
      'unitId': unit.id,
    }).query;
    return '/reader/${Uri.encodeComponent(unit.comicId)}?$query';
  }
}
