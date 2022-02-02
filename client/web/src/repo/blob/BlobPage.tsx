import classNames from 'classnames'
import * as H from 'history'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import React, { useState, useEffect, useCallback, useMemo } from 'react'
import { Redirect } from 'react-router'
import { Observable } from 'rxjs'
import { catchError, map, mapTo, startWith, switchMap } from 'rxjs/operators'

import { ErrorMessage } from '@sourcegraph/branded/src/components/alerts'
import { ErrorLike, isErrorLike, asError } from '@sourcegraph/common'
import { SearchContextProps } from '@sourcegraph/search'
import { StreamingSearchResultsListProps } from '@sourcegraph/search-ui'
import { ExtensionsControllerProps } from '@sourcegraph/shared/src/extensions/controller'
import { Scalars } from '@sourcegraph/shared/src/graphql-operations'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'
import { AbsoluteRepoFile, ModeSpec, parseQueryAndHash } from '@sourcegraph/shared/src/util/url'
import { Alert, Button, LoadingSpinner, useEventObservable } from '@sourcegraph/wildcard'

import { AuthenticatedUser } from '../../auth'
import { BreadcrumbSetters } from '../../components/Breadcrumbs'
import { HeroPage } from '../../components/HeroPage'
import { PageTitle } from '../../components/PageTitle'
import { SearchStreamingProps } from '../../search'
import { useSearchStack, useExperimentalFeatures } from '../../stores'
import { toTreeURL } from '../../util/url'
import { fetchRepository, resolveRevision } from '../backend'
import { FilePathBreadcrumbs } from '../FilePathBreadcrumbs'
import { HoverThresholdProps } from '../RepoContainer'
import { RepoHeaderContributionsLifecycleProps } from '../RepoHeader'
import { RepoHeaderContributionPortal } from '../RepoHeaderContributionPortal'

import { ToggleHistoryPanel } from './actions/ToggleHistoryPanel'
import { ToggleLineWrap } from './actions/ToggleLineWrap'
import { ToggleRenderedFileMode } from './actions/ToggleRenderedFileMode'
import { getModeFromURL } from './actions/utils'
import { fetchBlob } from './backend'
import { Blob, BlobInfo } from './Blob'
import styles from './BlobPage.module.scss'
import { GoToRawAction } from './GoToRawAction'
import { useBlobPanelViews } from './panel/BlobPanel'
import { RenderedFile } from './RenderedFile'
import { RenderedSearchNotebookMarkdown, SEARCH_NOTEBOOK_FILE_EXTENSION } from './RenderedSearchNotebookMarkdown'
import { BlobFileFields } from '../../graphql-operations'

interface Props
    extends AbsoluteRepoFile,
        ModeSpec,
        RepoHeaderContributionsLifecycleProps,
        SettingsCascadeProps,
        PlatformContextProps,
        TelemetryProps,
        ExtensionsControllerProps,
        ThemeProps,
        HoverThresholdProps,
        BreadcrumbSetters,
        SearchStreamingProps,
        Pick<SearchContextProps, 'searchContextsEnabled'>,
        Pick<StreamingSearchResultsListProps, 'fetchHighlightedFileLineRanges'> {
    location: H.Location
    history: H.History
    repoID: Scalars['ID']
    authenticatedUser: AuthenticatedUser | null
    globbing: boolean
    isMacPlatform: boolean
    isSourcegraphDotCom: boolean
    repoUrl: string
}

export const BlobPage: React.FunctionComponent<Props> = props => {
    const [wrapCode, setWrapCode] = useState(ToggleLineWrap.getValue())
    let renderMode = getModeFromURL(props.location)
    const { repoName, revision, commitID, filePath, isLightTheme, useBreadcrumb, mode, repoUrl } = props
    const showSearchNotebook = useExperimentalFeatures(features => features.showSearchNotebook)
    const showSearchContext = useExperimentalFeatures(features => features.showSearchContext ?? false)
    const lineOrRange = useMemo(() => parseQueryAndHash(props.location.search, props.location.hash), [
        props.location.search,
        props.location.hash,
    ])

    // Log view event whenever a new Blob, or a Blob with a different render mode, is visited.
    useEffect(() => {
        props.telemetryService.logViewEvent('Blob', { repoName, filePath })
    }, [repoName, commitID, filePath, renderMode, props.telemetryService])

    useSearchStack(
        useMemo(
            () => ({
                type: 'file',
                path: filePath,
                repo: repoName,
                revision,
                // Need to subtract 1 because IHighlightLineRange is 0-based but
                // line information in the URL is 1-based.
                lineRange: lineOrRange.line
                    ? { startLine: lineOrRange.line - 1, endLine: (lineOrRange.endLine ?? lineOrRange.line + 1) - 1 }
                    : null,
            }),
            [filePath, repoName, revision, lineOrRange.line, lineOrRange.endLine]
        )
    )

    useBreadcrumb(
        useMemo(() => {
            if (!filePath) {
                return
            }

            return {
                key: 'filePath',
                className: 'flex-shrink-past-contents',
                element: (
                    // TODO should these be "flattened" all using setBreadcrumb()?
                    <FilePathBreadcrumbs
                        key="path"
                        repoName={repoName}
                        revision={revision}
                        filePath={filePath}
                        isDir={false}
                        repoUrl={repoUrl}
                        telemetryService={props.telemetryService}
                    />
                ),
            }
        }, [filePath, revision, repoName, repoUrl, props.telemetryService])
    )

    // Bundle latest blob with all other file info to pass to `Blob`
    // Prevents https://github.com/sourcegraph/sourcegraph/issues/14965 by not allowing
    // components to use current file props while blob hasn't updated, since all information
    // is bundled in one object whose creation is blocked by `fetchBlob` emission.
    const [nextFetchWithDisabledTimeout, blobInfoOrError] = useEventObservable<
        void,
        (BlobInfo & { richHTML: string; aborted: boolean }) | null | ErrorLike
    >(
        useCallback(
            (clicks: Observable<void>) =>
                clicks.pipe(
                    mapTo(true),
                    startWith(false),
                    switchMap(disableTimeout =>
                        fetchBlob({
                            repoName,
                            commitID,
                            filePath,
                            disableTimeout,
                        })
                    ),
                    map(blob => {
                        if (blob === null) {
                            return blob
                        }
                        console.log({ lsif: blob.highlight.lsif })
                        renderLsifHtml(blob)

                        const blobInfo: BlobInfo & { richHTML: string; aborted: boolean } = {
                            content: blob.content,
                            html: blob.highlight.html,
                            repoName,
                            revision,
                            commitID,
                            filePath,
                            mode,
                            // Properties used in `BlobPage` but not `Blob`
                            richHTML: blob.richHTML,
                            aborted: blob.highlight.aborted,
                        }
                        return blobInfo
                    }),
                    catchError((error): [ErrorLike] => {
                        console.error(error)
                        return [asError(error)]
                    })
                ),
            [repoName, revision, commitID, filePath, mode]
        )
    )

    const onExtendTimeoutClick = useCallback(
        (event: React.MouseEvent): void => {
            event.preventDefault()
            nextFetchWithDisabledTimeout()
        },
        [nextFetchWithDisabledTimeout]
    )

    const getPageTitle = (): string => {
        const repoNameSplit = repoName.split('/')
        const repoString = repoNameSplit.length > 2 ? repoNameSplit.slice(1).join('/') : repoName
        if (filePath) {
            const fileOrDirectory = filePath.split('/').pop()!
            return `${fileOrDirectory} - ${repoString}`
        }
        return `${repoString}`
    }

    useBlobPanelViews(props)

    const isSearchNotebook =
        blobInfoOrError &&
        !isErrorLike(blobInfoOrError) &&
        blobInfoOrError.filePath.endsWith(SEARCH_NOTEBOOK_FILE_EXTENSION) &&
        showSearchNotebook

    // If url explicitly asks for a certain rendering mode, renderMode is set to that mode, else it checks:
    // - If file contains richHTML and url does not include a line number: We render in richHTML.
    // - If file does not contain richHTML or the url includes a line number: We render in code view.
    if (!renderMode) {
        renderMode =
            blobInfoOrError && !isErrorLike(blobInfoOrError) && blobInfoOrError.richHTML && !lineOrRange.line
                ? 'rendered'
                : 'code'
    }

    // Always render these to avoid UI jitter during loading when switching to a new file.
    const alwaysRender = (
        <>
            <PageTitle title={getPageTitle()} />
            <RepoHeaderContributionPortal
                position="right"
                priority={20}
                id="toggle-blob-panel"
                repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
            >
                {context => (
                    <ToggleHistoryPanel
                        {...context}
                        key="toggle-blob-panel"
                        location={props.location}
                        history={props.history}
                    />
                )}
            </RepoHeaderContributionPortal>
            {renderMode === 'code' && (
                <RepoHeaderContributionPortal
                    position="right"
                    priority={99}
                    id="toggle-line-wrap"
                    repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
                >
                    {context => <ToggleLineWrap {...context} key="toggle-line-wrap" onDidUpdate={setWrapCode} />}
                </RepoHeaderContributionPortal>
            )}
            <RepoHeaderContributionPortal
                position="right"
                priority={30}
                id="raw-action"
                repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
            >
                {context => (
                    <GoToRawAction
                        {...context}
                        telemetryService={props.telemetryService}
                        key="raw-action"
                        repoName={repoName}
                        revision={props.revision}
                        filePath={filePath}
                    />
                )}
            </RepoHeaderContributionPortal>
        </>
    )

    if (isErrorLike(blobInfoOrError)) {
        // Be helpful if the URL was actually a tree and redirect.
        // Some extensions may optimistically construct blob URLs because
        // they cannot easily determine eagerly if a file path is a tree or a blob.
        // We don't have error names on GraphQL errors.
        if (/not a blob/i.test(blobInfoOrError.message)) {
            return <Redirect to={toTreeURL(props)} />
        }
        return (
            <>
                {alwaysRender}
                <HeroPage icon={AlertCircleIcon} title="Error" subtitle={<ErrorMessage error={blobInfoOrError} />} />
            </>
        )
    }

    if (blobInfoOrError === undefined) {
        // Render placeholder for layout before content is fetched.
        return (
            <div className={styles.placeholder}>
                {alwaysRender}
                <div className="d-flex mt-3 justify-content-center">
                    <LoadingSpinner />
                </div>
            </div>
        )
    }

    // File not found:
    if (blobInfoOrError === null) {
        return (
            <div className={styles.placeholder}>
                <HeroPage
                    icon={MapSearchIcon}
                    title="Not found"
                    subtitle={`${filePath} does not exist at this revision.`}
                />
            </div>
        )
    }

    return (
        <>
            {alwaysRender}
            {blobInfoOrError.richHTML && (
                <RepoHeaderContributionPortal
                    position="right"
                    priority={100}
                    id="toggle-rendered-file-mode"
                    repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
                >
                    {({ actionType }) => (
                        <ToggleRenderedFileMode
                            key="toggle-rendered-file-mode"
                            mode={renderMode || 'rendered'}
                            actionType={actionType}
                        />
                    )}
                </RepoHeaderContributionPortal>
            )}
            {isSearchNotebook && renderMode === 'rendered' && (
                <RenderedSearchNotebookMarkdown
                    {...props}
                    markdown={blobInfoOrError.content}
                    resolveRevision={resolveRevision}
                    fetchRepository={fetchRepository}
                    showSearchContext={showSearchContext}
                />
            )}
            {!isSearchNotebook && blobInfoOrError.richHTML && renderMode === 'rendered' && (
                <RenderedFile dangerousInnerHTML={blobInfoOrError.richHTML} location={props.location} />
            )}
            {!blobInfoOrError.richHTML && blobInfoOrError.aborted && (
                <div>
                    <Alert variant="info">
                        Syntax-highlighting this file took too long. &nbsp;
                        <Button onClick={onExtendTimeoutClick} variant="primary" size="sm">
                            Try again
                        </Button>
                    </Alert>
                </div>
            )}
            {/* Render the (unhighlighted) blob also in the case highlighting timed out */}
            {renderMode === 'code' && (
                <Blob
                    className={classNames('test-repo-blob', styles.blob)}
                    blobInfo={blobInfoOrError}
                    wrapCode={wrapCode}
                    platformContext={props.platformContext}
                    extensionsController={props.extensionsController}
                    settingsCascade={props.settingsCascade}
                    onHoverShown={props.onHoverShown}
                    history={props.history}
                    isLightTheme={isLightTheme}
                    telemetryService={props.telemetryService}
                    location={props.location}
                />
            )}
        </>
    )
}

interface Document {
    occurrences: Occurrence[]
}
interface Occurrence {
    range: number[]
    syntaxKind: number
}
class Position {
    constructor(public readonly line: number, public readonly character: number) {}
}
class Range {
    constructor(public readonly start: Position, public readonly end: Position) {}
}


enum SyntaxKind {
  UnspecifiedSyntaxKind = 0,

  // Comment, including comment markers and text
  Comment = 1,

  // `,` `.` `,`
  PunctuationDelimiter = 2,
  // (), {}, [] when used syntactically
  PunctuationBracket = 3,

  // `if`, `else`, `return`, `class`, etc.
  IdentifierKeyword = 4,

  // `+`, `*`, etc.
  IdentifierOperator = 5,

  // non-specific catch-all for any identifier not better described elsewhere
  Identifier = 6,
  // Identifiers builtin to the language: `min`, `print` in Python.
  IdentifierBuiltin = 7,
  // Identifiers representing `null`-like values: `None` in Python, `nil` in Go.
  IdentifierNull = 8,
  // `xyz` in `const xyz = "hello"`
  IdentifierConstant = 9,
  // `var X = "hello"` in Go
  IdentifierMutableGlobal = 10,
  // both parameter definition and references
  IdentifierParameter = 11,
  // identifiers for variable definitions and references within a local scope
  IdentifierLocal = 12,
  // Used when identifier shadowes some other identifier within the scope
  IdentifierShadowed = 13,
  // `package main`
  IdentifierModule = 14,

  // Function call/reference
  IdentifierFunction = 15,
  // Function definition only
  IdentifierFunctionDefinition = 16,

  // Macro call/reference
  IdentifierMacro = 17,
  // Macro definition only
  IdentifierMacroDefinition = 18,

  // non-builtin types, including namespaces
  IdentifierType = 19,
  // builtin types only, such as `str` for Python or `int` in Go
  IdentifierBuiltinType = 20,

  // Python decorators, c-like __attribute__
  IdentifierAttribute = 21,

  // `\b`
  RegexEscape = 22,
  // `*`, `+`
  RegexRepeated = 23,
  // `.`
  RegexWildcard = 24,
  // `(`, `)`, `[`, `]`
  RegexDelimiter = 25,
  // `|`, `-`
  RegexJoin = 26,

  // Literal strings: "Hello, world!"
  StringLiteral = 27,
  // non-regex escapes: "\t", "\n"
  StringLiteralEscape = 28,
  // datetimes within strings, special words within a string, `{}` in format strings
  StringLiteralSpecial = 29,
  // "key" in { "key": "value" }, useful for example in JSON
  StringLiteralKey = 30,
  // 'c' or similar, in languages that differentiate strings and characters
  CharacterLiteral = 31,
  // Literal numbers, both floats and integers
  NumericLiteral = 32,
  // `true`, `false`
  BooleanLiteral = 33,

  // Used for XML-like tags
  Tag = 34,
  // Attribute name in XML-like tags
  TagAttribute = 35,
  // Delimiters for XML-like tags
  TagDelimiter = 36,
}

// const syntaxKinds = [
// // export enum SyntaxKind {
// //     UnspecifiedSyntaxKind = 0,
//   '',
// //     Operator = 1,
//   'hl-operator',
// //     Comment = 2,
//   'hl-comment',
// //     PunctuationDelimiter = 3,
//   'hl-punction hl-section hl-brackets',
// //     PunctuationBracket = 4,
//   'hl-punction hl-section hl-brackets',
// //     PunctuationSpecial = 5,
//   'hl-punction hl-section hl-brackets',
// //     Keyword = 6,
//   'hl-keyword',
// //     Identifier = 7,
//   'hl-variable',
// //     BuiltinIdentifier = 8,
//   'hl-variable',
// //     NullIdentifier = 9,
//   'hl-null',
// //     ConstantIdentifier = 10,
//   'hl-constant',
// //     MutableGlobalIdentifier = 11,
//   'hl-variable',
// //     ParameterIdentifier = 12,
//   'hl-variable',
// //     LocalIdentifier = 13,
//   'hl-variable',
// //     ShadowedIdentifier = 14,
//   'hl-variable',
// //     ModuleIdentifier = 15,
//   'hl-variable',
// //     MacroIdentifier = 16,
//   'hl-variable',
// //     StringLiteral = 17,
//   'hl-string',
// //     StringLiteralRegex = 18,
//   'hl-regexp hl-string',
// //     StringLiteralEscape = 19,
//   'hl-constant hl-character hl-escape',
// //     StringLiteralSpecial = 20,
//   'hl-constant hl-string',
// //     StringLiteralKey = 21,
//   'hl-constant hl-string',
// //     CharacterLiteral = 22,
//   'hl-constant hl-character',
// //     NumericLiteral = 23,
//   'hl-constant hl-numeric',
// //     BooleanLiteral = 24,
//   'hl-boolean',
// //     FunctionDefinition = 25,
//   'hl-entity hl-name hl-function',
// //     MacroDefinition = 26,
//   'hl-function',
// //     TypeIdentifier = 27,
//   'hl-storage hl-type',
// //     BuiltinTypeIdentifier = 28,
//   'hl-storage hl-type',
// //     AttributeIdentifier = 29,
//   'hl-attribute-name',
// //     Tag = 30,
//   'hl-tag',
// //     TagAttribute = 31,
//   'hl-tag',
// //     TagDelimiter = 32
//   'hl-tag'
// // }
// ]

function range(occurrence: Occurrence): Range {
    const start = new Position(occurrence.range[0], occurrence.range[1])
    const end =
        occurrence.range.length === 3
            ? new Position(occurrence.range[0], occurrence.range[2])
            : new Position(occurrence.range[2], occurrence.range[3])
    return new Range(start, end)
}

function renderLsifHtml(blob: BlobFileFields): void {
    if (blob.highlight.lsif) {
        const document = JSON.parse(blob.highlight.lsif) as Document
        const language = 'go'
        const lines = blob.content.replaceAll('\r\n', '\n').split('\n')
        const html = new HtmlBuilder()
        html.openTag('table')
        html.openTag('tbody')
        let documentIndex = 0
        for (const [lineNumber, line] of lines.entries()) {
            console.log({ line })
            html.openTag('tr')
            html.raw(`<td class="line" data-line="${lineNumber + 1}"></td>`)

            html.openTag('td class="code"')
            html.openTag('div')
            html.openTag(`span class="hl-source hl-${language}"`)
            let start = 0
            while (
                documentIndex < document.occurrences.length &&
                document.occurrences[documentIndex].range[0] === lineNumber
            ) {
                const occurrence = document.occurrences[documentIndex]
                const r = range(occurrence)
                let kind = SyntaxKind[occurrence.syntaxKind];
                if (occurrence.syntaxKind) {
                    html.plaintext(line.slice(start, r.start.character))
                    html.span(
                        `class="hl-typed-${kind}"`,
                        line.slice(r.start.character, r.end.character)
                    )
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
        blob.highlight.html = html.buffer.join('')
    }
}

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
