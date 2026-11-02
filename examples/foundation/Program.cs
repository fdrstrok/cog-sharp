using System.Text.Json;
using System.Text.Json.Nodes;
using D = Grafana.Foundation.Dashboardv2;
using L = Grafana.Foundation.Dashboard;
using P = Grafana.Foundation.Prometheus;
using T = Grafana.Foundation.Timeseries;
using C = Grafana.Foundation.Common;
using A = Grafana.Foundation.Alertingv0alpha1;
using V = Grafana.Foundation.Cog.Variants;

var dashboard = new D.DashboardBuilder("Service health")
    .Panel("requests", new D.PanelBuilder()
        .Title("Requests")
        .Visualization(new T.TimeseriesVisualizationV2Builder().Unit("reqps").LineWidth(2))
        .Data(new D.QueryGroupBuilder()
            .Target(new D.TargetBuilder()
                .Query(new P.PrometheusQueryV2Builder().Expr("sum(rate(http_requests_total[5m]))")))
            .Target(new D.TargetBuilder().RefId("B")
                .Query(new P.PrometheusQueryV2Builder().Expr("up")))))
    .AutoGridLayout(new D.AutoGridBuilder().WithItem("requests"))
    .Build();

var json = JsonSerializer.Serialize(dashboard, new JsonSerializerOptions { WriteIndented = true });
using (var document = JsonDocument.Parse(json))
{
    var panel = document.RootElement.GetProperty("elements").GetProperty("requests").GetProperty("spec");
    Require(panel.GetProperty("vizConfig").GetProperty("group").GetString() == "timeseries", "visualization discriminator");
    Require(
        panel.GetProperty("vizConfig").GetProperty("spec").GetProperty("fieldConfig")
            .GetProperty("defaults").GetProperty("custom").GetProperty("lineWidth").GetDouble() == 2,
        "composed visualization initialization and assignment");
    var queries = panel.GetProperty("data").GetProperty("spec").GetProperty("queries");
    Require(queries[0].GetProperty("spec").GetProperty("refId").GetString() == "query-0", "automatic query refId");
    Require(queries[1].GetProperty("spec").GetProperty("refId").GetString() == "B", "explicit query refId");
    Require(
        queries[0].GetProperty("spec").GetProperty("query").GetProperty("spec").GetProperty("expr").GetString()
            == "sum(rate(http_requests_total[5m]))",
        "composed query spec serialization");
}

var restored = JsonSerializer.Deserialize<D.Dashboard>(json)!;
var restoredPanel = restored.Elements["requests"].PanelKind!.Spec;
Require(restoredPanel.VizConfig.Spec.Options is T.Options, "typed visualization options deserialization");
Require(restoredPanel.VizConfig.Spec.FieldConfig.Defaults.Custom is T.FieldConfig { LineWidth: 2 },
    "typed visualization fields deserialization");
Require(restoredPanel.Data.Spec.Queries[0].Spec.Query.Spec is P.Dataquery,
    "typed query spec deserialization");
Require(JsonNode.DeepEquals(JsonNode.Parse(json), JsonSerializer.SerializeToNode(restored)), "dashboard wire round trip");
var collisionSafeQueries = new D.QueryGroupBuilder()
    .Target(new D.TargetBuilder().RefId("query-0"))
    .Target(new D.TargetBuilder())
    .Build().Spec.Queries;
Require(collisionSafeQueries[1].Spec.RefId == "query-1", "automatic refId skips explicit IDs");
var batchQueries = new D.QueryGroupBuilder()
    .Targets([new D.TargetBuilder(), new D.TargetBuilder().RefId("query-0")])
    .Build().Spec.Queries;
Require(batchQueries[0].Spec.RefId == "query-1", "batch refIds reserve explicit IDs first");
try
{
    new D.QueryGroupBuilder().Targets([new D.TargetBuilder().RefId("A"), new D.TargetBuilder().RefId("A")]);
    throw new InvalidOperationException("Duplicate query refIds were accepted");
}
catch (ArgumentException)
{
}
Require(new D.Dashboard().CursorSync == D.DashboardCursorSync.Off, "schema enum default");
Require(new D.TimeSettingsSpec().AutoRefreshIntervals is { Count: 10 } intervals && intervals[0] == "5s",
    "schema array defaults");
Require(JsonSerializer.Serialize(L.VariableRefresh.OnDashboardLoad) == "1", "numeric enum wire value");
Require(JsonSerializer.Serialize(JsonSerializer.Deserialize<P.ResultAssertionsType>("\"\"")) == "\"\"",
    "empty string enum member round trip");

var unknown = JsonSerializer.Deserialize<D.DataQueryKind>(
    """{"kind":"DataQuery","group":"custom-plugin","version":"v0","spec":{"flag":false,"count":0,"nested":{"x":1}}}""")!;
Require(unknown.Spec is JsonElement, "unknown query stays raw JSON");
using (var document = JsonDocument.Parse(JsonSerializer.Serialize(unknown)))
{
    Require(document.RootElement.GetProperty("spec").GetProperty("flag").GetBoolean() == false, "unknown query false value");
    Require(document.RootElement.GetProperty("spec").GetProperty("nested").GetProperty("x").GetInt32() == 1,
        "unknown query nested data");
}

var unknownViz = JsonSerializer.Deserialize<D.VizConfigKind>(
    """{"kind":"VizConfig","group":"custom-panel","version":"v0","spec":{"options":{"enabled":false},"fieldConfig":{"defaults":{"custom":{"size":0}},"overrides":[]}}}""")!;
Require(unknownViz.Spec.Options is JsonElement, "unknown panel options remain raw JSON");
using (var document = JsonDocument.Parse(JsonSerializer.Serialize(unknownViz)))
{
    Require(document.RootElement.GetProperty("spec").GetProperty("options").GetProperty("enabled").GetBoolean() == false,
        "unknown panel option retained");
    Require(document.RootElement.GetProperty("spec").GetProperty("fieldConfig").GetProperty("defaults")
        .GetProperty("custom").GetProperty("size").GetInt32() == 0, "unknown panel custom fields retained");
}

var legacy = new L.DashboardBuilder("Legacy")
    .WithPanel(new T.TimeseriesPanelBuilder().Title("First").Span(12).Height(8)
        .WithTarget(new P.DataqueryBuilder().Expr("up")
            .Datasource(new C.DataSourceRef { Type = "prometheus", Uid = "prom" })))
    .WithPanel(new T.TimeseriesPanelBuilder().Title("Second").Span(12).Height(8))
    .WithPanel(new T.TimeseriesPanelBuilder().Title("Third").Span(12).Height(8))
    .WithRow(new L.RowBuilder("Next row"))
    .Build();
Require(legacy.Panels![0].Panel!.GridPos is { X: 0, Y: 0 }, "first legacy panel position");
Require(legacy.Panels[1].Panel!.GridPos is { X: 12, Y: 0 }, "next legacy panel position");
Require(legacy.Panels[2].Panel!.GridPos is { X: 0, Y: 8 }, "legacy grid wraps");
Require(legacy.Panels[3].RowPanel!.GridPos is { Y: 16, W: 24, H: 1 }, "row follows partially filled line");
var unequalSpans = new L.DashboardBuilder("Unequal spans")
    .WithPanel(new T.TimeseriesPanelBuilder().Span(16).Height(8))
    .WithPanel(new T.TimeseriesPanelBuilder().Span(12).Height(4))
    .Build();
Require(unequalSpans.Panels![1].Panel!.GridPos is { X: 0, Y: 8 }, "panels wrap before exceeding grid width");
var legacyRestored = JsonSerializer.Deserialize<L.Dashboard>(JsonSerializer.Serialize(legacy))!;
Require(legacyRestored.Panels![0].Panel!.Targets![0] is P.Dataquery { Expr: "up" },
    "legacy query variant serialization");
Require(legacyRestored.Panels[0].Panel!.Options is T.Options, "legacy panel options deserialization");

V.Dataquery rawQuery = JsonSerializer.Deserialize<V.Dataquery>("""{"custom":{"enabled":false},"refId":"Z"}""")!;
Require(rawQuery is V.UnknownDataquery, "unknown legacy query");
Require(JsonNode.DeepEquals(JsonNode.Parse("""{"custom":{"enabled":false},"refId":"Z"}"""),
    JsonSerializer.SerializeToNode(rawQuery)), "unknown legacy query round trip");

var facets = new A.SearchResultsBuilder()
    .Facets(new Dictionary<string, List<Grafana.Foundation.Cog.IBuilder<A.FacetValue>>>
    {
        ["team"] = [new A.FacetValueBuilder().Value("ops").Count(3)],
    }).Build();
Require(facets.Facets!["team"][0].Count == 3, "map of builder arrays");

if (args.Length != 0)
{
    File.WriteAllText(args[0], json);
}
Console.WriteLine("Full SDK: Dashboardv2, typed query/viz JSON, unknown variants, legacy layout, enums, and nested builders passed.");

static void Require(bool condition, string name)
{
    if (!condition)
    {
        throw new InvalidOperationException($"Failed: {name}");
    }
}
