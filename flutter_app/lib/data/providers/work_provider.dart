import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/library_api.dart';
import '../api/work_api.dart';
import '../models/work.dart';

class WorkListState {
  final List<Work> works;
  final WorkQuery query;
  final bool isLoading;
  final bool isLoadingMore;
  final int total;
  final String? error;

  const WorkListState({
    this.works = const [],
    this.query = const WorkQuery(),
    this.isLoading = false,
    this.isLoadingMore = false,
    this.total = 0,
    this.error,
  });

  bool get hasMore => works.length < total;

  WorkListState copyWith({
    List<Work>? works,
    WorkQuery? query,
    bool? isLoading,
    bool? isLoadingMore,
    int? total,
    String? error,
    bool clearError = false,
  }) =>
      WorkListState(
        works: works ?? this.works,
        query: query ?? this.query,
        isLoading: isLoading ?? this.isLoading,
        isLoadingMore: isLoadingMore ?? this.isLoadingMore,
        total: total ?? this.total,
        error: clearError ? null : error ?? this.error,
      );
}

class WorkListNotifier extends StateNotifier<WorkListState> {
  final Ref _ref;

  WorkListNotifier(this._ref) : super(const WorkListState());

  Future<void> load({WorkQuery? query}) async {
    final requested = (query ?? state.query).copyWith(page: 1);
    state = state.copyWith(
      query: requested,
      isLoading: true,
      clearError: true,
    );
    try {
      final response = await _ref.read(workApiProvider).listWorks(requested);
      state = state.copyWith(
        works: response.works,
        total: response.total,
        isLoading: false,
      );
    } catch (error) {
      state = state.copyWith(isLoading: false, error: error.toString());
    }
  }

  Future<void> loadMore() async {
    if (state.isLoadingMore || !state.hasMore) return;
    final next = state.query.copyWith(page: state.query.page + 1);
    state = state.copyWith(isLoadingMore: true);
    try {
      final response = await _ref.read(workApiProvider).listWorks(next);
      state = state.copyWith(
        query: next,
        works: [...state.works, ...response.works],
        total: response.total,
        isLoadingMore: false,
      );
    } catch (error) {
      state = state.copyWith(isLoadingMore: false, error: error.toString());
    }
  }

  Future<void> toggleFavorite(Work work) async {
    final next = !work.isFavorite;
    await _ref.read(workApiProvider).setFavorite(work.id, next);
    state = state.copyWith(
      works: state.works
          .map((item) => item.id == work.id
              ? Work.fromJson({..._workToJson(item), 'isFavorite': next})
              : item)
          .toList(),
    );
  }
}

Map<String, dynamic> _workToJson(Work work) => {
      'id': work.id,
      'libraryId': work.libraryId,
      'title': work.title,
      'rootPath': work.rootPath,
      'representativeComicId': work.representativeComicId,
      'coverComicId': work.coverComicId,
      'coverUrl': work.coverUrl,
      'coverAspectRatio': work.coverAspectRatio,
      'itemCount': work.itemCount,
      'completedItemCount': work.completedItemCount,
      'pageCount': work.pageCount,
      'fileSize': work.fileSize,
      'totalReadTime': work.totalReadTime,
      'addedAt': work.addedAt,
      'updatedAt': work.updatedAt,
      'sortOrder': work.sortOrder,
      'author': work.author,
      'publisher': work.publisher,
      'year': work.year,
      'description': work.description,
      'language': work.language,
      'genre': work.genre,
      'status': work.status,
      'metadataSource': work.metadataSource,
      'externalRating': work.externalRating,
      'externalRatingMax': work.externalRatingMax,
      'externalRatingSource': work.externalRatingSource,
      'tags': work.tags
          .map((tag) => {'id': tag.id, 'name': tag.name, 'color': tag.color})
          .toList(),
      'categories': work.categories
          .map((category) => {
                'id': category.id,
                'name': category.name,
                'slug': category.slug,
                'icon': category.icon,
              })
          .toList(),
      'isFavorite': work.isFavorite,
      'rating': work.rating,
      'readingStatus': work.readingStatus,
      'lastReadAt': work.lastReadAt,
      'continueComicId': work.continueComicId,
      'continuePage': work.continuePage,
      'continueUnitId': work.continueUnitId,
      'units': work.units
          .map((unit) => {
                'id': unit.id,
                'workId': unit.workId,
                'comicId': unit.comicId,
                'title': unit.title,
                'displayLabel': unit.displayLabel,
                'startPage': unit.startPage,
                'pageCount': unit.pageCount,
                'sortIndex': unit.sortIndex,
                'coverUrl': unit.coverUrl,
              })
          .toList(),
    };

final workListProvider =
    StateNotifierProvider<WorkListNotifier, WorkListState>((ref) {
  return WorkListNotifier(ref);
});

final workDetailProvider = FutureProvider.family<Work, String>((ref, id) {
  return ref.read(workApiProvider).getWork(id);
});

final accessibleLibrariesProvider = FutureProvider<List<LibrarySummary>>((ref) {
  return ref.read(libraryApiProvider).listAccessible();
});
