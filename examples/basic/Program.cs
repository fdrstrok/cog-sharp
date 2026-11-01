using System;
using System.Text.Json;
using Example.Foundation.Demo;
using Example.Foundation.Shared;

var dashboard = new DashboardBuilder()
    .Title("CPU")
    .Tags(["production"])
    .Child(new ChildBuilder().Enabled(false))
    .Remote(new RemoteChildBuilder().Label("remote"))
    .Build();

var json = JsonSerializer.Serialize(dashboard);
var restored = JsonSerializer.Deserialize<Dashboard>(json)!;
Require(
    restored.Title == "CPU" &&
    restored.Tags[0] == "production" &&
    restored.Child is { Enabled: false } &&
    restored.Remote?.Label == "remote",
    "default JSON round trip and fluent builders");

using (var document = JsonDocument.Parse(json))
{
    Require(!document.RootElement.TryGetProperty("active", out _), "unset optional bool is omitted");
    Require(!document.RootElement.TryGetProperty("count", out _), "unset optional number is omitted");
}

var explicitDefaults = JsonSerializer.Deserialize<Dashboard>(
    """{"title":"defaults","tags":[],"value":false,"active":false,"count":0}""")!;
using (var document = JsonDocument.Parse(JsonSerializer.Serialize(explicitDefaults)))
{
    Require(document.RootElement.GetProperty("value").GetBoolean() == false, "false union branch is retained");
    Require(document.RootElement.GetProperty("active").GetBoolean() == false, "explicit false is retained");
    Require(document.RootElement.GetProperty("count").GetInt64() == 0, "explicit zero is retained");
}

var tagged = JsonSerializer.Deserialize<Dashboard>(
    """{"title":"union","tags":[],"choice":{"kind":"first","value":"ok"}}""")!;
Require(tagged.Choice?.First?.Value == "ok", "discriminator selects the first branch after plugin RPC");
using (var document = JsonDocument.Parse(JsonSerializer.Serialize(tagged)))
{
    Require(document.RootElement.GetProperty("choice").GetProperty("kind").GetString() == "first", "tagged union wire shape");
}

var keywords = new KeywordsBuilder()
    .String("123\"")
    .Bool(false)
    .Event(new ChildBuilder().Enabled(false))
    .Build();
Require(
    keywords.String == "123\"" && !keywords.Bool && !keywords.Event.Enabled,
    "keyword parameters and nested-builder variable names");

try
{
    new KeywordsBuilder().String("letters");
    throw new InvalidOperationException("Regexp constraint accepted an invalid value");
}
catch (ArgumentException error)
{
    Require(error.Message.Contains("""^\d+"?$"""), "escaped regexp exception message");
}

Console.WriteLine(json);
Console.WriteLine("C# compilation, builders, default JSON, optional values, union RPC, foreign references, and regexp checks passed.");

static void Require(bool condition, string name)
{
    if (!condition)
    {
        throw new InvalidOperationException($"Failed: {name}");
    }
}
