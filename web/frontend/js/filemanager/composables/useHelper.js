import { useSettingsStore } from '../stores/useSettingsStore.js'

/**
 * Helper composable for common utility functions
 * Replaces helper.js mixin
 */
export function useHelper() {
    const settings = useSettingsStore()

    function bytesToHuman(bytes) {
        const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']

        if (bytes === 0) return '0 Bytes'

        const i = parseInt(Math.floor(Math.log(bytes) / Math.log(1024)), 10)

        if (i === 0) return `${bytes} ${sizes[i]}`

        return `${(bytes / 1024 ** i).toFixed(1)} ${sizes[i]}`
    }

    function timestampToDate(timestamp) {
        if (timestamp === undefined || timestamp === null) return '-'

        const date = new Date(timestamp * 1000)

        return date.toLocaleString(settings.lang)
    }

    function mimeToIcon(mime) {
        const mimeTypes = {
            'image/gif': 'file-image',
            'image/png': 'file-image',
            'image/jpeg': 'file-image',
            'image/bmp': 'file-image',
            'image/webp': 'file-image',
            'image/tiff': 'file-image',
            'image/svg+xml': 'file-image',
            'text/plain': 'file-lines',
            'text/javascript': 'file-code',
            'application/json': 'file-code',
            'text/markdown': 'file-code',
            'text/html': 'file-code',
            'text/css': 'file-code',
            'audio/midi': 'file-audio',
            'audio/mpeg': 'file-audio',
            'audio/webm': 'file-audio',
            'audio/ogg': 'file-audio',
            'audio/wav': 'file-audio',
            'audio/aac': 'file-audio',
            'audio/x-wav': 'file-audio',
            'audio/mp4': 'file-audio',
            'video/webm': 'file-video',
            'video/ogg': 'file-video',
            'video/mpeg': 'file-video',
            'video/3gpp': 'file-video',
            'video/x-flv': 'file-video',
            'video/mp4': 'file-video',
            'video/quicktime': 'file-video',
            'video/x-msvideo': 'file-video',
            'video/vnd.dlna.mpeg-tts': 'file-video',
            'application/x-bzip': 'file-archive',
            'application/x-bzip2': 'file-archive',
            'application/x-tar': 'file-archive',
            'application/gzip': 'file-archive',
            'application/zip': 'file-archive',
            'application/x-7z-compressed': 'file-archive',
            'application/x-rar-compressed': 'file-archive',
            'application/pdf': 'file-pdf',
            'application/rtf': 'file',
            'application/msword': 'file',
            'application/vnd.ms-word': 'file',
            'application/vnd.ms-excel': 'file',
            'application/vnd.ms-powerpoint': 'file',
            'application/vnd.oasis.opendocument.text': 'file',
            'application/vnd.oasis.opendocument.spreadsheet': 'file',
            'application/vnd.oasis.opendocument.presentation': 'file',
            'application/vnd.openxmlformats-officedocument.wordprocessingml': 'file',
            'application/vnd.openxmlformats-officedocument.spreadsheetml': 'file',
            'application/vnd.openxmlformats-officedocument.presentationml': 'file',
        }

        if (mimeTypes[mime] !== undefined) {
            return mimeTypes[mime]
        }

        return 'file'
    }

    function extensionToIcon(extension) {
        const extensionTypes = {
            gif: 'file-image',
            png: 'file-image',
            jpeg: 'file-image',
            jpg: 'file-image',
            bmp: 'file-image',
            psd: 'file-image',
            svg: 'file-image',
            ico: 'file-image',
            ai: 'file-image',
            tif: 'file-image',
            tiff: 'file-image',
            webp: 'file-image',
            txt: 'file-lines',
            json: 'file-lines',
            log: 'file-lines',
            ini: 'file-lines',
            xml: 'file-lines',
            md: 'file-lines',
            env: 'file-lines',
            js: 'file-code',
            php: 'file-code',
            css: 'file-code',
            cpp: 'file-code',
            class: 'file-code',
            h: 'file-code',
            java: 'file-code',
            sh: 'file-code',
            swift: 'file-code',
            aif: 'file-audio',
            cda: 'file-audio',
            mid: 'file-audio',
            mp3: 'file-audio',
            mpa: 'file-audio',
            ogg: 'file-audio',
            wav: 'file-audio',
            wma: 'file-audio',
            wmv: 'file-video',
            avi: 'file-video',
            mpeg: 'file-video',
            mpg: 'file-video',
            flv: 'file-video',
            mp4: 'file-video',
            mkv: 'file-video',
            mov: 'file-video',
            ts: 'file-video',
            '3gpp': 'file-video',
            zip: 'file-archive',
            arj: 'file-archive',
            deb: 'file-archive',
            pkg: 'file-archive',
            rar: 'file-archive',
            rpm: 'file-archive',
            '7z': 'file-archive',
            'tar.gz': 'file-archive',
            pdf: 'file-pdf',
            rtf: 'file',
            doc: 'file',
            docx: 'file',
            odt: 'file',
            xlr: 'file',
            xls: 'file',
            xlsx: 'file',
            ppt: 'file',
            pptx: 'file',
            pptm: 'file',
            xps: 'file',
            potx: 'file',
        }

        if (extension && extensionTypes[extension.toLowerCase()] !== undefined) {
            return extensionTypes[extension.toLowerCase()]
        }

        return 'file'
    }

    function addRenameSuffix(name, taken) {
        const dot = name.lastIndexOf('.')
        const base = dot > 0 ? name.slice(0, dot) : name
        const ext = dot > 0 ? name.slice(dot) : ''
        let i = 1
        let candidate = `${base}_${i}${ext}`
        while (taken.has(candidate)) {
            i += 1
            candidate = `${base}_${i}${ext}`
        }

        return candidate
    }

    return {
        bytesToHuman,
        timestampToDate,
        mimeToIcon,
        extensionToIcon,
        addRenameSuffix,
    }
}
