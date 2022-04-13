package search

type JSONHeatmapFacetMap JSONFacetMap

func CreateJSONHeatmapFacetMap(fieldname string) *JSONHeatmapFacetMap {
	jfm := CreateJSONFacetMap(fieldname)
	return (*JSONHeatmapFacetMap)(jfm)
}

func (jfm *JSONHeatmapFacetMap) withSubFacet(string, *JSONFacetMap) *JSONHeatmapFacetMap {
	panic("subfacets not supported in Heatmap")
}

/**
 * Indicate the region to compute the heatmap facet on.
 *
 * Defaults to the "world" ("[-180,-90 TO 180,90]")
 */
func (jfm *JSONHeatmapFacetMap) SetRegionQuery(queryString string) *JSONHeatmapFacetMap {
	(*jfm)["geom"] = queryString
	return jfm
}

/**
 * Indicates the size of each cell in the computed heatmap grid
 *
 * If not set, defaults to being computed by {@code distErrPct} or {@code distErr}
 *
 * @param individualCellSize the forced size of each cell in the heatmap grid
 *
 * @see #setDistErr(double)
 * @see #setDistErrPct(double)
 */
func (jfm *JSONHeatmapFacetMap) SetGridLevel(individualCellSize int64) *JSONHeatmapFacetMap {
	if individualCellSize <= 0 {
		individualCellSize = 1
	}
	(*jfm)["gridLevel"] = individualCellSize

	return jfm
}

/**
 * A fraction of the heatmap region that is used to compute the cell size.
 *
 * Defaults to 0.15 if not specified.
 *
 * @see #setGridLevel(int)
 * @see #setDistErr(double)
 */
func (jfm *JSONHeatmapFacetMap) SetDistErrPct(distErrPct float64) *JSONHeatmapFacetMap {
	if distErrPct < 0 {
		distErrPct = 0
	}
	if distErrPct > 1 {
		distErrPct = 1
	}
	(*jfm)["distErrPct"] = distErrPct

	return jfm
}

/**
 * Indicates the maximum acceptable cell error distance.
 *
 * Used to compute the size of each cell in the heatmap grid rather than specifying {@link #setGridLevel(int)}
 *
 * @param distErr a positive value representing the maximum acceptable cell error.
 *
 * @see #setGridLevel(int)
 * @see #setDistErrPct(double)
 */
func (jfm *JSONHeatmapFacetMap) SetDistErr(distErr float64) *JSONHeatmapFacetMap {
	if distErr < 0 {
		distErr = 0
	}
	(*jfm)["distErr"] = distErr
	return jfm
}

/*
public enum HeatmapFormat {
INTS2D("ints2D"), PNG("png");

private final String value;

HeatmapFormat(String value) {
this.value = value;
}

@Override
public String toString() { return value; }
}
*/

/**
 * Sets the format that the computed heatmap should be returned in.
 *
 * Defaults to 'ints2D' if not specified.
 */
func (jfm *JSONHeatmapFacetMap) SetHeatmapFormat(format string) *JSONHeatmapFacetMap {
	(*jfm)["format"] = format
	return jfm
}
