import GET from './get.js'

const LIST_BATCH = 5

function joinPath(...parts) {
    return parts
        .filter((p) => p !== null && p !== undefined && p !== '')
        .join('/')
        .replace(/\/+/g, '/')
}

function dirOf(relPath) {
    const i = relPath.lastIndexOf('/')

    return i === -1 ? '' : relPath.slice(0, i)
}

function uniqueDirs(files, emptyDirs) {
    const dirs = new Set([''])
    for (const f of files) {
        let d = dirOf(f.relPath)
        while (d) {
            dirs.add(d)
            const i = d.lastIndexOf('/')
            d = i === -1 ? '' : d.slice(0, i)
        }
    }
    for (const d of emptyDirs) {
        let cur = d
        while (cur) {
            dirs.add(cur)
            const i = cur.lastIndexOf('/')
            cur = i === -1 ? '' : cur.slice(0, i)
        }
    }

    return dirs
}

async function fetchListing(disk, path) {
    try {
        const response = await GET.content(disk, path || null, { silent: true })
        if (!response || !response.data) return null
        if (response.data.result && response.data.result.status !== 'success') return null
        const items = new Map()
        const directories = response.data.directories || []
        const filesArr = response.data.files || []
        for (const d of directories) items.set(d.basename, 'dir')
        for (const f of filesArr) items.set(f.basename, 'file')

        return items
    } catch (err) {
        return null
    }
}

async function batchedListings(disk, dirs) {
    const result = new Map()
    const list = Array.from(dirs)
    let cursor = 0
    async function worker() {
        while (cursor < list.length) {
            const idx = cursor
            cursor += 1
            const relDir = list[idx]
            const listing = await fetchListing(disk, relDir)
            result.set(relDir, listing)
        }
    }
    const concurrency = Math.min(LIST_BATCH, list.length || 1)
    await Promise.all(Array.from({ length: concurrency }, () => worker()))

    return result
}

function exists(listings, dir, basename) {
    const listing = listings.get(dir)
    if (!listing) return null

    return listing.get(basename) || null
}

function dirExistsOnServer(listings, dirRel) {
    if (dirRel === '') return true
    const parent = dirOf(dirRel)
    const parentListing = listings.get(parent)
    if (!parentListing) return false
    const last = dirRel.slice(parent ? parent.length + 1 : 0)
    const kind = parentListing.get(last)

    return kind === 'dir'
}

export async function detectConflicts({ disk, rootPath, files, emptyDirs }) {
    const result = {
        fileConflicts: new Map(),
        dirConflicts: new Map(),
        listings: null,
    }

    const dirsToList = uniqueDirs(files, emptyDirs)
    const absDirs = new Set()
    for (const rel of dirsToList) {
        absDirs.add(joinPath(rootPath || '', rel))
    }
    const listings = await batchedListings(disk, absDirs)
    result.listings = listings

    const relToAbs = (rel) => joinPath(rootPath || '', rel)

    for (const f of files) {
        const fileDir = dirOf(f.relPath)
        const absParent = relToAbs(fileDir)
        const listing = listings.get(absParent)
        if (!listing) {
            result.fileConflicts.set(f.relPath, 'none')
            continue
        }
        const existing = listing.get(f.name)
        if (!existing) {
            result.fileConflicts.set(f.relPath, 'none')
        } else if (existing === 'dir') {
            result.fileConflicts.set(f.relPath, 'dir-vs-file')
        } else {
            result.fileConflicts.set(f.relPath, 'file')
        }
    }

    for (const d of emptyDirs) {
        const parent = dirOf(d)
        const absParent = relToAbs(parent)
        const last = d.slice(parent ? parent.length + 1 : 0)
        const listing = listings.get(absParent)
        if (!listing) {
            result.dirConflicts.set(d, 'none')
            continue
        }
        const existing = listing.get(last)
        if (!existing) {
            result.dirConflicts.set(d, 'none')
        } else if (existing === 'file') {
            result.dirConflicts.set(d, 'file-vs-dir')
        } else {
            result.dirConflicts.set(d, 'merge')
        }
    }

    return result
}

export { joinPath, dirOf, uniqueDirs, dirExistsOnServer, exists }
