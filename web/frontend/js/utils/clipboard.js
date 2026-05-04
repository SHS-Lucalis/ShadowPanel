export async function copyToClipboard(text) {
    if (window.isSecureContext && navigator.clipboard?.writeText) {
        try {
            await navigator.clipboard.writeText(text)

            return true
        } catch {
            // fall through to legacy fallback
        }
    }

    return legacyCopy(text)
}

function legacyCopy(text) {
    const textArea = document.createElement('textarea')
    textArea.value = text
    textArea.setAttribute('readonly', '')
    textArea.style.position = 'fixed'
    textArea.style.top = '0'
    textArea.style.left = '0'
    textArea.style.width = '1px'
    textArea.style.height = '1px'
    textArea.style.padding = '0'
    textArea.style.border = 'none'
    textArea.style.outline = 'none'
    textArea.style.boxShadow = 'none'
    textArea.style.background = 'transparent'
    textArea.style.opacity = '0'
    document.body.appendChild(textArea)

    const previousActive = document.activeElement
    const previousSelection = document.getSelection()
    const savedRange = previousSelection && previousSelection.rangeCount > 0
        ? previousSelection.getRangeAt(0)
        : null

    let succeeded = false
    try {
        textArea.focus()
        textArea.select()
        textArea.setSelectionRange(0, text.length)
        succeeded = document.execCommand('copy')
    } catch {
        succeeded = false
    } finally {
        document.body.removeChild(textArea)
        if (savedRange && previousSelection) {
            previousSelection.removeAllRanges()
            previousSelection.addRange(savedRange)
        }
        if (previousActive && typeof previousActive.focus === 'function') {
            previousActive.focus()
        }
    }

    return succeeded
}
