import {
    StreamDownloadError,
    baseUrlToAbsolute,
    streamSaveResponse,
} from './download-stream.js'

export { StreamDownloadError as FileDownloadError }

const buildFileURL = (baseUrl, disk, path) => {
    const url = new URL(`${baseUrlToAbsolute(baseUrl)}/download`)
    url.searchParams.set('disk', disk)
    url.searchParams.set('path', path)

    return url
}

export async function downloadSingleFile({
    baseUrl,
    disk,
    path,
    filename,
    headers,
    onPhase,
    onProgress,
    signal,
}) {
    if (onPhase) onPhase('preparing')

    const url = buildFileURL(baseUrl, disk, path)

    const handleResponse = () => {
        if (onPhase) onPhase('downloading')
    }

    const result = await streamSaveResponse({
        url,
        filename,
        headers,
        signal,
        onResponse: handleResponse,
        onProgress,
    })

    if (onPhase) onPhase('completed')

    return { totalBytes: result.total }
}
