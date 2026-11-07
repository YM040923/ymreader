import io.github.keiyoushi.gradle.api.ContentWarning

plugins {
    alias(kei.plugins.extension)
}

keiyoushi {
    name = "折角"
    versionCode = 3
    contentWarning = ContentWarning.SAFE
    libVersion = "1.6"

    source {
        name = "折角"
        lang = "zh"
        baseUrl = "http://47.102.212.49:6680"
        id = 0x6E6F77656E7A6AL
    }
}
