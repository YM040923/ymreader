package eu.kanade.tachiyomi.extension.zh.nowen

import android.content.SharedPreferences
import androidx.preference.EditTextPreference
import androidx.preference.PreferenceScreen
import eu.kanade.tachiyomi.source.ConfigurableSource
import eu.kanade.tachiyomi.source.model.Filter
import eu.kanade.tachiyomi.source.model.FilterList
import eu.kanade.tachiyomi.source.model.MangasPage
import eu.kanade.tachiyomi.source.model.Page
import eu.kanade.tachiyomi.source.model.SChapter
import eu.kanade.tachiyomi.source.model.SManga
import eu.kanade.tachiyomi.source.model.SMangaUpdate
import keiyoushi.annotation.Source
import keiyoushi.network.Fetch
import keiyoushi.network.get
import keiyoushi.source.KeiSource
import keiyoushi.utils.getPreferencesLazy
import keiyoushi.utils.parseAs
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.encodeToJsonElement
import okhttp3.Headers
import okhttp3.Request
import java.net.URLEncoder
import kotlin.random.Random

/**
 * 折角（NowenReader）原生源。
 *
 * 直接读后端 /api/works、/api/works/:id/units 和 /api/comics/:id/page/:n，
 * 不经过 Komga/OPDS 兼容层；Work -> 漫画，Unit -> 章节。
 */
@Source
class NowenReader(
    override val lang: String,
    override val id: Long,
) : KeiSource(),
    ConfigurableSource {

    internal val preferences: SharedPreferences by getPreferencesLazy()

    override val name = "折角"
    override val supportsLatest = true
    override val supportsFilterFetching = true
    override val filterFetchHint: String get() = "点一下「重置」加载库和标签"

    override val baseUrl: String
        get() = preferences
            .getString(PREF_URL, DEFAULT_URL)!!
            .trim()
            .trimEnd('/')
            .ifBlank { DEFAULT_URL }

    private val apiKey: String
        get() = preferences.getString(PREF_API_KEY, "")!!.trim()

    override fun Headers.Builder.configureHeaders() = apply {
        set("User-Agent", "NowenReader/1.0")
        if (apiKey.isNotBlank()) {
            set("Authorization", "Bearer $apiKey")
        }
    }

    // ------------------------------------------------------------------
    // 浏览 / 搜索
    // ------------------------------------------------------------------

    override suspend fun getPopularManga(page: Int): MangasPage {
        // Fushi 发现页的「热门推荐」取每个来源热门行的第一条；如果热门行固定按
        // 标题排序，首条永远是同一本。这里一次取一批后打乱，让每次刷新首条都变。
        if (page > 1) return MangasPage(emptyList(), hasNextPage = false)
        val data = fetchWorksPage(
            page = 1,
            query = "",
            options = WorkQueryOptions(sortBy = "title", sortOrder = "asc"),
            pageSize = POPULAR_PAGE_SIZE,
        )
        val shuffled = data.works.shuffled().toMutableList()
        if (shuffled.size > 1) {
            // 尽量保证和上一次的首条不同，避免随机到同一本时看起来像没刷新。
            val last = preferences.getString(PREF_LAST_POPULAR, null)
            if (shuffled.first().id == last) {
                val index = 1 + Random.nextInt(shuffled.size - 1)
                val first = shuffled[0]
                shuffled[0] = shuffled[index]
                shuffled[index] = first
            }
            preferences.edit().putString(PREF_LAST_POPULAR, shuffled.first().id).apply()
        }
        return MangasPage(
            mangas = shuffled.map { it.toSManga() },
            hasNextPage = false,
        )
    }

    override suspend fun getLatestUpdates(page: Int): MangasPage = fetchWorks(page, query = "", options = WorkQueryOptions(sortBy = "addedAt", sortOrder = "desc"))

    override suspend fun getSearchMangaList(
        page: Int,
        query: String,
        filters: FilterList,
    ): MangasPage = fetchWorks(page, query = query, options = parseFilters(filters))

    private suspend fun fetchWorks(
        page: Int,
        query: String,
        options: WorkQueryOptions,
    ): MangasPage {
        val data = fetchWorksPage(page, query, options, PAGE_SIZE)
        return MangasPage(
            mangas = data.works.map { it.toSManga() },
            hasNextPage = page < data.totalPages,
        )
    }

    private suspend fun fetchWorksPage(
        page: Int,
        query: String,
        options: WorkQueryOptions,
        pageSize: Int,
    ): WorksResponse {
        val url = buildString {
            append(baseUrl)
            append("/api/works?page=").append(page)
            append("&pageSize=").append(pageSize)
            append("&sortBy=").append(options.sortBy)
            append("&sortOrder=").append(options.sortOrder)
            if (!options.libraryId.isNullOrBlank()) {
                append("&libraryId=").append(encode(options.libraryId))
            }
            if (options.favoritesOnly) {
                append("&favorites=true")
            }
            if (options.tags.isNotEmpty()) {
                append("&tags=").append(encode(options.tags.joinToString(",")))
            }
            if (query.isNotBlank()) {
                append("&search=").append(encode(query))
            }
        }
        return client.get(url, Fetch.XHR).parseAs<WorksResponse>()
    }

    // ------------------------------------------------------------------
    // 筛选器：全部 / 国漫 / 日漫 / 收藏 / 标签 / 排序
    // ------------------------------------------------------------------

    override suspend fun fetchFilterData(): JsonElement {
        val libraries = client.get("$baseUrl/api/libraries/accessible", Fetch.XHR)
            .parseAs<LibrariesResponse>()
            .libraries
            .filter { it.enabled && it.name.isNotBlank() }
            .distinctBy { it.id }
            .sortedBy { it.name }

        val tags = client.get("$baseUrl/api/works/tags", Fetch.XHR)
            .parseAs<TagsResponse>()
            .tags
            .filter { it.name.isNotBlank() }
            .distinctBy { it.name }
            .sortedWith(compareByDescending<TagStatDto> { it.count }.thenBy { it.name })

        return json.encodeToJsonElement(FilterPayload(libraries, tags))
    }

    override fun getFilterList(data: JsonElement?): FilterList {
        val payload = data?.let {
            runCatching { json.decodeFromJsonElement<FilterPayload>(it) }.getOrNull()
        } ?: FilterPayload()

        val filters = mutableListOf<Filter<*>>()
        filters += LibrarySelect(payload.libraries)
        filters += FavoriteSelect()
        filters += SortSelect()
        if (payload.tags.isNotEmpty()) {
            filters += Filter.Header("标签（可多选，命中任意一个）")
            payload.tags.forEach { filters += TagCheckBox(it.name) }
        }
        return FilterList(filters)
    }

    private fun parseFilters(filters: FilterList): WorkQueryOptions {
        val libraryId = filters.filterIsInstance<LibrarySelect>().firstOrNull()?.libraryId
        val favoritesOnly = filters.filterIsInstance<FavoriteSelect>().firstOrNull()?.onlyFavorites == true
        val tags = filters
            .filterIsInstance<TagCheckBox>()
            .filter { it.state }
            .map { it.tagName }
        val sort = filters.filterIsInstance<SortSelect>().firstOrNull()?.sort
            ?: SortSelect.DEFAULT
        return WorkQueryOptions(
            libraryId = libraryId,
            favoritesOnly = favoritesOnly,
            tags = tags,
            sortBy = sort.first,
            sortOrder = sort.second,
        )
    }

    override suspend fun fetchMangaUpdate(
        manga: SManga,
        chapters: List<SChapter>,
        fetchDetails: Boolean,
        fetchChapters: Boolean,
    ): SMangaUpdate {
        val newManga = if (fetchDetails) {
            client.get("$baseUrl/api/works/${manga.url}", Fetch.XHR)
                .parseAs<WorkDto>()
                .toSManga()
        } else {
            manga
        }
        val newChapters = if (fetchChapters) loadChapters(newManga) else chapters
        return SMangaUpdate(newManga, newChapters)
    }

    private suspend fun loadChapters(manga: SManga): List<SChapter> {
        val data = client.get("$baseUrl/api/works/${manga.url}/units", Fetch.XHR)
            .parseAs<UnitsResponse>()
        return data.units.map { unit ->
            SChapter.create().apply {
                url = "${unit.comicId}/${unit.startPage}/${unit.pageCount}"
                name = unit.displayLabel.ifBlank { unit.title }
                // 用后端排序号做章节号，保证番外/卷/正篇的展示顺序和折角一致。
                chapter_number = unit.sortIndex.toFloat()
                date_upload = 0L
                scanlator = ""
            }
        }
    }

    override suspend fun getPageList(chapter: SChapter): List<Page> {
        // 新章节用 "comicId/start/count"，旧章节可能是 "comicId|start|count"。
        val parts = chapter.url.split('/', '|').filter { it.isNotBlank() }
        if (parts.size < 3) return emptyList()
        val comicId = parts[parts.size - 3]
        val start = parts[parts.size - 2].toIntOrNull() ?: return emptyList()
        val count = parts[parts.size - 1].toIntOrNull() ?: return emptyList()
        return (0 until count).map { index ->
            val url = "$baseUrl/api/comics/$comicId/page/${start + index}"
            Page(index, url = url, imageUrl = url)
        }
    }

    override fun getImageRequest(page: Page): Request {
        val url = page.imageUrl ?: page.url
        val builder = super.getImageRequest(page).newBuilder().url(url)
        // 关键：没有 Key 时不要发 "Authorization: Bearer "。
        // 服务端看到 Authorization 头会优先按 API Key 校验，失败也不再回退 Cookie，
        // 空 Bearer 会让已经登录的 nowen_session 也一起失效（图片 401）。
        if (apiKey.isNotBlank()) {
            builder.header("Authorization", "Bearer $apiKey")
        }
        return builder.build()
    }

    override fun setupPreferenceScreen(screen: PreferenceScreen) {
        EditTextPreference(screen.context).apply {
            key = PREF_URL
            title = "服务器地址"
            summary = baseUrl
            setDefaultValue(DEFAULT_URL)
            dialogTitle = "服务器地址"
            setOnPreferenceChangeListener { pref, value ->
                pref.summary = value.toString()
                true
            }
        }.let(screen::addPreference)

        EditTextPreference(screen.context).apply {
            key = PREF_API_KEY
            title = "API Key（可选）"
            summary = if (apiKey.isBlank()) "未设置（使用内置浏览器登录的 Cookie）" else "已设置"
            setDefaultValue("")
            dialogTitle = "API Key"
            setOnPreferenceChangeListener { pref, value ->
                pref.summary = if (value.toString().isBlank()) {
                    "未设置（使用内置浏览器登录的 Cookie）"
                } else {
                    "已设置"
                }
                true
            }
        }.let(screen::addPreference)
    }

    private fun WorkDto.toSManga(): SManga = SManga.create().apply {
        url = id
        title = this@toSManga.title
        author = this@toSManga.author
        description = this@toSManga.description
        genre = buildList {
            if (this@toSManga.genre.isNotBlank()) add(this@toSManga.genre)
            val tagNames = tags.map { it.name }.filter { it.isNotBlank() }
            if (tagNames.isNotEmpty()) add(tagNames.joinToString(", "))
        }.joinToString(", ")
        status = when {
            this@toSManga.status.contains("完") -> SManga.COMPLETED
            this@toSManga.status.contains("连载") || this@toSManga.status.contains("連載") ->
                SManga.ONGOING
            else -> SManga.UNKNOWN
        }
        thumbnail_url = resolveUrl(
            coverUrl?.takeIf { it.isNotBlank() }
                ?: coverComicId?.takeIf { it.isNotBlank() }?.let { "/api/comics/$it/thumbnail" },
        )
        initialized = true
    }

    private fun resolveUrl(raw: String?): String? {
        val value = raw?.trim().orEmpty()
        if (value.isEmpty()) return null
        if (value.startsWith("http://") || value.startsWith("https://")) return value
        return baseUrl + (if (value.startsWith("/")) "" else "/") + value
    }

    private fun encode(value: String): String = URLEncoder.encode(value, "UTF-8")

    private data class WorkQueryOptions(
        val libraryId: String? = null,
        val favoritesOnly: Boolean = false,
        val tags: List<String> = emptyList(),
        val sortBy: String = "title",
        val sortOrder: String = "asc",
    )

    private class LibrarySelect(libraries: List<LibraryDto>) :
        Filter.Select<String>(
            "内容",
            arrayOf("全部") + libraries.map { it.name }.toTypedArray(),
        ) {
        private val ids: List<String?> = listOf<String?>(null) + libraries.map { it.id }

        val libraryId: String?
            get() = ids.getOrNull(state)
    }

    private class FavoriteSelect :
        Filter.Select<String>(
            "收藏",
            arrayOf("全部", "仅收藏"),
        ) {
        val onlyFavorites: Boolean
            get() = state == 1
    }

    private class SortSelect :
        Filter.Select<String>(
            "排序",
            arrayOf(
                "标题 A→Z",
                "最近添加",
                "最近更新",
                "上次阅读",
                "评分",
            ),
        ) {
        val sort: Pair<String, String>
            get() = when (state) {
                1 -> "addedAt" to "desc"
                2 -> "updatedAt" to "desc"
                3 -> "lastReadAt" to "desc"
                4 -> "rating" to "desc"
                else -> DEFAULT
            }

        companion object {
            val DEFAULT: Pair<String, String> = "title" to "asc"
        }
    }

    private class TagCheckBox(name: String) : Filter.CheckBox(name, false) {
        val tagName: String = name
    }

    companion object {
        private const val DEFAULT_URL = "http://47.102.212.49:6680"
        private const val PAGE_SIZE = 24
        private const val POPULAR_PAGE_SIZE = 300
        private const val PREF_URL = "server_url"
        private const val PREF_API_KEY = "api_key"
        private const val PREF_LAST_POPULAR = "last_popular_first"

        private val json = Json {
            ignoreUnknownKeys = true
            isLenient = true
        }
    }
}

@Serializable
private data class WorksResponse(
    val works: List<WorkDto> = emptyList(),
    val total: Int = 0,
    val totalPages: Int = 0,
)

@Serializable
private data class WorkDto(
    val id: String = "",
    val title: String = "",
    val author: String = "",
    val description: String = "",
    val genre: String = "",
    val status: String = "",
    val coverUrl: String? = null,
    val coverComicId: String? = null,
    val year: Int? = null,
    val tags: List<TagDto> = emptyList(),
)

@Serializable
private data class TagDto(val name: String = "")

@Serializable
private data class UnitsResponse(val units: List<UnitDto> = emptyList())

@Serializable
private data class UnitDto(
    val id: String = "",
    val title: String = "",
    val displayLabel: String = "",
    val comicId: String = "",
    val startPage: Int = 0,
    val pageCount: Int = 0,
    val sortIndex: Int = 0,
)

@Serializable
private data class LibrariesResponse(val libraries: List<LibraryDto> = emptyList())

@Serializable
private data class LibraryDto(
    val id: String = "",
    val name: String = "",
    val enabled: Boolean = true,
)

@Serializable
private data class TagsResponse(val tags: List<TagStatDto> = emptyList())

@Serializable
private data class TagStatDto(
    val name: String = "",
    val count: Int = 0,
)

@Serializable
private data class FilterPayload(
    val libraries: List<LibraryDto> = emptyList(),
    val tags: List<TagStatDto> = emptyList(),
)
