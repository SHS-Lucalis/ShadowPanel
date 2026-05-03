import {
    StreamDownloadError,
    baseUrlToAbsolute,
    parseIntHeader,
    streamSaveResponse,
} from './download-stream.js'

export { StreamDownloadError as ArchiveDownloadError }

const buildArchiveURL = (baseUrl, disk, path, compress) => {
    const url = new URL(`${baseUrlToAbsolute(baseUrl)}/download-archive`)
    url.searchParams.set('disk', disk)
    url.searchParams.set('path', path)
    if (compress && compress > 0) {
        url.searchParams.set('compress', String(compress))
    }

    return url
}

export async function downloadDirectoryArchive({
    baseUrl,
    disk,
    path,
    filename,
    compress,
    headers,
    onPhase,
    onProgress,
    signal,
}) {
    if (onPhase) onPhase('preparing')

    const url = buildArchiveURL(baseUrl, disk, path, compress)
    let totalFiles = 0
    let skippedCount = 0

    const handleResponse = (response) => {
        totalFiles = parseIntHeader(response, 'X-Archive-Total-Files')
        skippedCount = parseIntHeader(response, 'X-Archive-Skipped-Count')
        if (onPhase) onPhase('downloading')
    }

    const wrappedProgress = onProgress
        ? ({ loaded, total }) => onProgress({ loaded, total, totalFiles, skippedCount })
        : undefined

    const result = await streamSaveResponse({
        url,
        filename,
        headers,
        sizeHeader: 'X-Archive-Total-Bytes',
        signal,
        onResponse: handleResponse,
        onProgress: wrappedProgress,
    })

    if (onPhase) onPhase('completed')

    return { totalFiles, totalBytes: result.total, skippedCount }
}
