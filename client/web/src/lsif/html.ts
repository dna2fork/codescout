import { Document, Occurrence, Position, Range, SyntaxKind } from './spec';

class HtmlBuilder {
    public readonly buffer: string[] = []
    public plaintext(value: string): void {
        this.span('', value)
    }
    public span(attributes: string, value: string): void {
        this.element('span', attributes, value)
    }
    public element(element: string, attributes: string, value: string): void {
        this.openTag(element + ' ' + attributes)
        this.raw(value)
        this.closeTag(element)
    }
    public raw(html: string): void {
        this.buffer.push(html)
    }
    public openTag(tag: string): void {
        this.buffer.push('<')
        this.buffer.push(tag)
        this.buffer.push('>')
    }
    public closeTag(tag: string): void {
        this.buffer.push('</')
        this.buffer.push(tag)
        this.buffer.push('>')
    }
}

function range(occurrence: Occurrence): Range {
    const start = new Position(occurrence.range[0], occurrence.range[1])
    const end =
        occurrence.range.length === 3
            ? new Position(occurrence.range[0], occurrence.range[2])
            : new Position(occurrence.range[2], occurrence.range[3])
    return new Range(start, end)
}

export function render(lsif_json: string, content: string): string {
    const lsif_document = JSON.parse(lsif_json) as Document
    const language = 'go'
    const lines = content.replaceAll('\r\n', '\n').split('\n')
    const html = new HtmlBuilder()
    html.openTag('table')
    html.openTag('tbody')
    let documentIndex = 0
    for (const [lineNumber, line] of lines.entries()) {
        html.openTag('tr')
        html.raw(`<td class="line" data-line="${lineNumber + 1}"></td>`)

        html.openTag('td class="code"')
        html.openTag('div')
        html.openTag(`span class="hl-source hl-${language}"`)
        let start = 0
        while (
            documentIndex < lsif_document.occurrences.length &&
            lsif_document.occurrences[documentIndex].range[0] === lineNumber
        ) {
            const occurrence = lsif_document.occurrences[documentIndex]
            const r = range(occurrence)
            let kind = SyntaxKind[occurrence.syntaxKind]
            if (occurrence.syntaxKind) {
                html.plaintext(line.slice(start, r.start.character))
                html.span(`class="hl-typed-${kind}"`, line.slice(r.start.character, r.end.character))
                start = r.end.character
            }
            documentIndex++
        }
        html.plaintext(line.slice(start))
        html.closeTag('span')
        html.closeTag('div')
        html.closeTag('td')

        html.closeTag('tr')
    }
    html.closeTag('tbody')
    html.closeTag('table')

    return html.buffer.join('')
}
