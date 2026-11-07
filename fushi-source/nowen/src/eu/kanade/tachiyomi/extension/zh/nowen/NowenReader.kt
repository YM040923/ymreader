package eu.kanade.tachiyomi.extension.zh.nowen

import android.content.SharedPreferences
import androidx.preference.EditTextPreference
import androidx.preference.PreferenceScreen
import eu.kanade.tachiyomi.source.ConfigurableSource
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
import okhttp3.Headers
import okhttp3.Request
import java.net.URLEncoder

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

    override suspend fun getPopularManga(page: Int): MangasPage = fetchWorks(page, query = "", sortBy = "title", sortOrder = "asc")

    override suspend fun getLatestUpdates(page: Int): MangasPage = fetchWorks(page, query = "", sortBy = "addedAt", sortOrder = "desc")

    override suspend fun getSearchMangaList(
        page: Int,
        query: String,
        filters: FilterList,
    ): MangasPage = fetchWorks(page, query = query, sortBy = "title", sortOrder = "asc")

    private suspend fun fetchWorks(
        page: Int,
        query: String,
        sortBy: String,
        sortOrder: String,
    ): MangasPage {
        val url = buildString {
            append(baseUrl)
            append("/api/works?page=").append(page)
            append("&pageSize=").append(PAGE_SIZE)
            append("&sortBy=").append(sortBy)
            append("&sortOrder=").append(sortOrder)
            if (query.isNotBlank()) {
                append("&search=").append(URLEncoder.encode(query, "UTF-8"))
            }
        }
        val data = client.get(url, Fetch.XHR).parseAs<WorksResponse>()
        return MangasPage(
            mangas = data.works.map { it.toSManga() },
            hasNextPage = page < data.totalPages,
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
                url = "${unit.comicId}|${unit.startPage}|${unit.pageCount}"
                name = unit.displayLabel.ifBlank { unit.title }
                // 用后端排序号做章节号，保证番外/卷/正篇的展示顺序和折角一致。
                chapter_number = unit.sortIndex.toFloat()
                date_upload = 0L
                scanlator = ""
            }
        }
    }

    override suspend fun getPageList(chapter: SChapter): List<Page> {
        val parts = chapter.url.split("|")
        if (parts.size != 3) return emptyList()
        val comicId = parts[0]
        val start = parts[1].toIntOrNull() ?: return emptyList()
        val count = parts[2].toIntOrNull() ?: return emptyList()
        return (0 until count).map { index ->
            val url = "$baseUrl/api/comics/$comicId/page/${start + index}"
            Page(index, url = url, imageUrl = url)
        }
    }

    override fun getImageRequest(page: Page): Request {
        val url = page.imageUrl ?: page.url
        return super.getImageRequest(page).newBuilder()
            .url(url)
            .header("Authorization", "Bearer $apiKey")
            .build()
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
            title = "API Key"
            summary = if (apiKey.isBlank()) "未设置" else "已设置"
            setDefaultValue("")
            dialogTitle = "API Key"
            setOnPreferenceChangeListener { pref, value ->
                pref.summary = if (value.toString().isBlank()) "未设置" else "已设置"
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

    companion object {
        private const val DEFAULT_URL = "http://47.102.212.49:6680"
        private const val PAGE_SIZE = 24
        private const val PREF_URL = "server_url"
        private const val PREF_API_KEY = "api_key"
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
