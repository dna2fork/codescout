package reader

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif_typed"
)

func ConvertFlatToGraph(index *lsif_typed.Index) []Element {
	g := newGraph()

	g.emitVertex(
		"metaData",
		MetaData{
			Version:          "0.4.3",
			ProjectRoot:      index.Metadata.ProjectRoot,
			PositionEncoding: "utf-16",
			ToolInfo: ToolInfo{
				Name:    index.Metadata.ToolInfo.Name,
				Version: index.Metadata.ToolInfo.Version,
			},
		},
	)

	// Pass 1: build symbol table.
	for _, importedSymbol := range index.ExternalSymbols {
		g.symbolToResultIDs[importedSymbol.Symbol] = g.emitResults(importedSymbol, "import")
	}
	for _, document := range index.Documents {
		for _, exportedSymbol := range document.Symbols {
			if lsif_typed.IsGlobalSymbol(exportedSymbol.Symbol) {
				g.symbolToResultIDs[exportedSymbol.Symbol] = g.emitResults(exportedSymbol, "export")
			}
		}
	}

	// Pass 2: emit ranges.
	for _, document := range index.Documents {
		g.emitDocument(index, document)
	}

	return g.Elements
}

type ResultIDs struct {
	ResultSet            int
	DefinitionResult     int
	ReferenceResult      int
	ImplementationResult int
	HoverResult          int
	IsExported           bool
}

type graph struct {
	ID                  int
	Elements            []Element
	symbolToResultIDs   map[string]ResultIDs
	flatToGraphPackage  map[string]int
	monikerToKindToGID  map[string]map[string]int
	monikerToDefinition map[string][]RangeDoc
}

type RangeDoc struct {
	rnge int
	doc  int
}

func newGraph() graph {
	return graph{
		ID:                  0,
		Elements:            []Element{},
		symbolToResultIDs:   map[string]ResultIDs{},
		flatToGraphPackage:  map[string]int{},
		monikerToKindToGID:  map[string]map[string]int{},
		monikerToDefinition: map[string][]RangeDoc{},
	}
}

func (g *graph) emitPackage(pkg *lsif_typed.Package) int {
	id := pkg.ID()
	graphID, ok := g.flatToGraphPackage[id]
	if ok {
		return graphID
	}

	graphID = g.emitVertex("packageInformation", PackageInformation{
		Name:    pkg.Name,
		Version: pkg.Version,
	})
	g.flatToGraphPackage[pkg.ID()] = graphID
	return graphID
}

func (g *graph) resultIDs(symbol string, localSymtab map[string]ResultIDs) (ResultIDs, bool) {
	symtab := g.symbolToResultIDs
	if lsif_typed.IsLocalSymbol(symbol) {
		symtab = localSymtab
	}
	ids, ok := symtab[symbol]
	return ids, ok
}

func (g *graph) emitDocument(index *lsif_typed.Index, doc *lsif_typed.Document) {
	uri := index.Metadata.ProjectRoot + "/" + doc.RelativePath
	documentID := g.emitVertex("document", uri)
	localResultIDs := map[string]ResultIDs{}
	symtab := map[string]*lsif_typed.SymbolInformation{}
	for _, info := range doc.Symbols {
		symtab[info.Symbol] = info
		if lsif_typed.IsLocalSymbol(info.Symbol) {
			localResultIDs[info.Symbol] = g.emitResults(info, "")
		}
	}

	var rangeIDs []int
	for _, occ := range doc.Occurrences {
		rangeID, err := g.emitRange(occ.Range)
		if err != nil {
			// Silently skip invalid ranges.
			continue
		}
		rangeIDs = append(rangeIDs, rangeID)
		resultIDs, ok := g.resultIDs(occ.Symbol, localResultIDs)
		if !ok {
			// Silently skip occurrences to symbols with no matching SymbolInformation.
			continue
		}
		g.emitEdge("next", Edge{OutV: rangeID, InV: resultIDs.ResultSet})
		isDefinition := occ.SymbolRoles&int32(lsif_typed.SymbolRole_Definition) != 0
		if isDefinition {
			g.emitEdge("item", Edge{OutV: resultIDs.DefinitionResult, InVs: []int{rangeID}, Document: documentID})
			symbolInfo, ok := symtab[occ.Symbol]
			if ok {
				g.emitReferenceResults(rangeID, documentID, occ, resultIDs, localResultIDs, symbolInfo)
			}
		} else { // reference
			g.emitEdge("item", Edge{OutV: resultIDs.ReferenceResult, InVs: []int{rangeID}, Document: documentID})
		}
	}
	g.emitEdge("contains", Edge{OutV: documentID, InVs: rangeIDs})
}

func (g *graph) emitReferenceResults(rangeID, documentID int, occ *lsif_typed.Occurrence, resultIDs ResultIDs, localResultIDs map[string]ResultIDs, info *lsif_typed.SymbolInformation) {
	var allReferenceResultIds []int
	for _, relationship := range info.Relationships {
		if relationship.IsImplementation {
			g.emitEdge("item", Edge{OutV: resultIDs.ImplementationResult, InVs: []int{rangeID}, Document: documentID})
		}
		if relationship.IsReference {
			referenceResultIDs, ok := g.resultIDs(occ.Symbol, localResultIDs)
			if ok {
				allReferenceResultIds = append(allReferenceResultIds, referenceResultIDs.ReferenceResult)
				g.emitEdge("item", Edge{
					OutV:     referenceResultIDs.ReferenceResult,
					InVs:     []int{rangeID},
					Document: documentID,
					// The 'property' field is included in the LSIF Graph JSON but it's not present in reader.Element
					// Property: "referenceResults",
				})
			}
		}
	}
	g.emitEdge("item", Edge{
		OutV:     resultIDs.ReferenceResult,
		InVs:     allReferenceResultIds,
		Document: documentID,
		// The 'property' field is included in the LSIF Graph JSON but it's not present in reader.Element
		// Property: "referenceResults",
	})
}

func (g *graph) emitResults(info *lsif_typed.SymbolInformation, monikerKind string) ResultIDs {

	hover := strings.Join(info.Documentation, "\n\n---\n\n")
	definitionResult := -1
	isExported := monikerKind == "export"
	if isExported {
		definitionResult = g.emitVertex("definitionResult", nil)
	}
	implementationResult := -1
	if isExported {
		implementationResult = g.emitVertex("implementationResult", nil)
	}
	ids := ResultIDs{
		ResultSet:            g.emitVertex("resultSet", ResultSet{}),
		DefinitionResult:     definitionResult,
		ReferenceResult:      g.emitVertex("referenceResult", nil),
		ImplementationResult: implementationResult,
		HoverResult:          g.emitVertex("hoverResult", hover),
		IsExported:           isExported,
	}
	if isExported {
		g.emitEdge("textDocument/definition", Edge{OutV: ids.ResultSet, InV: ids.DefinitionResult})
		g.emitEdge("textDocument/implementation", Edge{OutV: ids.ResultSet, InV: ids.ImplementationResult})
	}
	g.emitEdge("textDocument/references", Edge{OutV: ids.ResultSet, InV: ids.ReferenceResult})
	g.emitEdge("textDocument/hover", Edge{OutV: ids.ResultSet, InV: ids.HoverResult})
	if monikerKind == "export" || monikerKind == "import" {
		g.emitMoniker(info.Symbol, monikerKind, ids.ResultSet)
	}

	return ids
}

func (g *graph) emitMoniker(symbolID string, kind string, resultSetID int) {
	symbol, err := lsif_typed.ParsePartialSymbol(symbolID, false)
	if err == nil && symbol != nil && symbol.Scheme != "" {
		// Accept the symbol as long as it has a non-empty scheme. We ignore
		// parse errors because we can still provide accurate
		// definition/references/hover within a repo.
		monikerID := g.emitVertex("moniker", Moniker{
			Kind:       kind,
			Scheme:     symbol.Scheme,
			Identifier: symbolID,
		})
		g.emitEdge("moniker", Edge{OutV: resultSetID, InV: monikerID})
		if symbol.Package != nil &&
			symbol.Package.Manager != "" &&
			symbol.Package.Name != "" &&
			symbol.Package.Version != "" {
			packageID := g.emitPackage(symbol.Package)
			g.emitEdge("packageInformation", Edge{OutV: monikerID, InV: packageID})
		}
	}
}
func (g *graph) emitRange(lsifRange []int32) (int, error) {
	startLine, startCharacter, endLine, endCharacter, err := interpretLsifRange(lsifRange)
	if err != nil {
		return 0, err
	}
	return g.emit("vertex", "range", Range{
		RangeData: protocol.RangeData{
			Start: protocol.Pos{
				Line:      int(startLine),
				Character: int(startCharacter),
			},
			End: protocol.Pos{
				Line:      int(endLine),
				Character: int(endCharacter),
			},
		},
	}), nil
}
func interpretLsifRange(rnge []int32) (startLine, startCharacter, endLine, endCharacter int32, err error) {
	if len(rnge) == 3 {
		return rnge[0], rnge[1], rnge[0], rnge[2], nil
	}
	if len(rnge) == 4 {
		return rnge[0], rnge[1], rnge[2], rnge[3], nil
	}
	return 0, 0, 0, 0, errors.Newf("invalid LSIF range %v", rnge)
}

func (g *graph) emitVertex(label string, payload interface{}) int {
	return g.emit("vertex", label, payload)
}

func (g *graph) emitEdge(label string, payload Edge) int {
	return g.emit("edge", label, payload)
}

func (g *graph) emit(ty, label string, payload interface{}) int {
	g.ID++
	g.Elements = append(g.Elements, Element{
		ID:      g.ID,
		Type:    ty,
		Label:   label,
		Payload: payload,
	})
	return g.ID
}

func WriteNDJSON(elements []interface{}, out io.Writer) error {
	w := writer.NewJSONWriter(out)
	for _, e := range elements {
		w.Write(e)
	}
	return w.Flush()
}

func ElementsToEmptyInterfaces(els []Element) []interface{} {
	var r []interface{}
	for _, el := range els {
		object := map[string]interface{}{
			"id":    el.ID,
			"type":  el.Type,
			"label": el.Label,
		}
		switch el.Label {
		case "hoverResult":
			object["result"] = map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": el.Payload,
				},
			}
		case "document":
			object["uri"] = el.Payload
		default:
			if el.Payload != nil {
				payload, err := json.Marshal(el.Payload)
				if err != nil {
					panic(err)
				}
				var x interface{}
				err = json.Unmarshal(payload, &x)
				if err != nil {
					panic(err)
				}
				if x != nil {
					for key, value := range x.(map[string]interface{}) {
						object[key] = value
					}
				}
			}
		}
		r = append(r, object)
	}
	return r
}
