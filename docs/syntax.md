# Supported syntax

This page lists the Mermaid syntax boxmaid understands, diagram by diagram. The Go package and the Rust crate parse the same syntax and produce the same output.

boxmaid reads Mermaid source and draws only what fits in a terminal. Syntax it doesn't understand is usually skipped without an error, so a diagram that renders in Mermaid will normally still render here, just with less in it. Each section ends with the syntax that is accepted but has no effect.

- [Common to all diagrams](#common-to-all-diagrams)
- Graph diagrams: [flowchart](#flowchart), [state](#state-diagram), [class](#class-diagram), [ER](#entity-relationship-diagram), [C4](#c4), [requirement](#requirement-diagram), [architecture](#architecture)
- Other diagrams: [sequence](#sequence-diagram), [block](#block-diagram), [gantt](#gantt), [journey](#user-journey), [timeline](#timeline), [mindmap](#mindmap), [git graph](#git-graph), [pie](#pie), [quadrant](#quadrant-chart), [XY chart](#xy-chart), [kanban](#kanban), [packet](#packet), [sankey](#sankey), [radar](#radar), [treemap](#treemap)

## Common to all diagrams

### Diagram header

- The first word of the first non-blank line picks the diagram type. A trailing `;` or `:` is ignored (`gitGraph:` works).
- A header that isn't listed on this page returns an `UnsupportedError`. An empty source returns `ErrEmpty`.
- Windows line endings are accepted. Tabs count as four spaces, which matters for diagrams that use indentation (mindmap, kanban, treemap).

### Front matter, directives and comments

- Front matter: a `---` block at the very start of the source. Only `title:` is read; surrounding quotes are removed. Any other key, including `config:`, is ignored.
- If the front matter has a title, it replaces a title given inside the diagram (`title …` in a gantt, pie and so on).
- `%%{init: …}%%` directives are removed wherever they are, including ones that span several lines. Their settings (theme, layout options, …) have no effect.
- `%%` comments are removed when they take a whole line. A `%%` after other text on the same line is not treated as a comment.

### Labels

These rules apply to node, edge, message and note text in flowchart, state, class, ER (relationship labels), C4, sequence, mindmap, block and kanban (card text). The other diagrams print their text as written.

- A label wrapped in `"…"` or `` `…` `` has the quotes or backticks removed, so Markdown strings such as `` "`**bold** text`" `` work.
- `<br>`, `<br/>` and `<br />` (any case) and a literal `\n` start a new line.
- Any other HTML tag (`<b>`, `<i>`, `<span …>`, …) is removed and its text kept.
- Mermaid's entity codes are decoded: named ones such as `#quot;` and `#amp;`, and decimal ones such as `#35;`.
- HTML entities are decoded: `&amp;`, `&lt;`, `&gt;`, `&quot;`, `&nbsp;`, `&rarr;` and other common named entities, plus all decimal (`&#35;`) and hex (`&#x23;`) entities. An unknown named entity is left as written.
- Markdown bold and italic markers are removed: `**bold**`, `__bold__`, `*italic*`, `_italic_`.
- Font Awesome icons are dropped: `fa:fa-car`, `fab:fa-github`, `fas:…`, `far:…`, `fal:…`, `fa-solid:…`, `fa-regular:…`.
- Runs of spaces are collapsed and each line is trimmed.

### Colours

- Only flowcharts use the diagram's own colours: `classDef`, `class`, `:::name` and `style`. See [Flowchart](#flowchart).
- Each node gets one colour, which applies to its border. boxmaid uses `stroke`, unless it is black, white or grey, and then `fill`, unless that is grey too. `color:` (text colour) is ignored.
- Accepted colour values are `#rgb`, `#rgba`, `#rrggbb` and `#rrggbbaa` (the alpha is dropped), plus these names: red, green, blue, orange, yellow, purple, pink, cyan, teal, lightblue, lightgreen, gold, violet, magenta. Other names and `rgb(…)` are ignored.
- Everywhere else `classDef`, `class` and `style` are skipped. A few charts (git graph, XY chart, sankey, radar, treemap) colour their series from a fixed built-in palette.

### Fitting the width

`Options.Width` is the number of columns available; `0` means unlimited.

- Graph diagrams (flowchart, state, class, ER, C4, requirement and architecture) wrap node labels at 28 columns, then 20, then 14, in the diagram's own direction. If none of those fit, boxmaid tries the other direction: `LR`/`RL` diagrams are laid out top to bottom, and `TB`/`BT` diagrams left to right, again at 28, 20 and 14. It uses the first layout that fits. If nothing fits, it returns the narrowest layout and does not report an error.
- Sequence diagrams wrap message labels at 40, then 28, 20 and 14 columns, and return the narrowest if none fits.
- Charts and lists (gantt, pie, journey, timeline, mindmap, kanban, quadrant, XY chart, packet, sankey, radar, treemap) size their bars, columns or text to the width.
- Block diagrams and git graphs are drawn at their natural size.
- `Options.ASCII` draws with plain ASCII characters instead of Unicode box drawing.

## Flowchart

Headers: `flowchart`, `graph`, `flowchart-elk`.

- Direction after the header: `TB`, `TD`, `BT`, `LR`, `RL`, or `>` (LR), `<` (RL), `^` (BT), `v` (TB). The default is `TB`.
- Statements can be separated by `;`, including on the header line (`graph LR; A-->B`).
- Node shapes:
  - `A[text]` rectangle
  - `A(text)` rounded
  - `A([text])` stadium
  - `A[[text]]` subroutine
  - `A[(text)]` cylinder
  - `A((text))` circle
  - `A(((text)))` double circle
  - `A>text]` asymmetric (drawn as a rectangle)
  - `A{text}` rhombus
  - `A{{text}}` hexagon
  - `A[/text/]` and `A[\text\]` parallelograms
  - `A[/text\]` and `A[\text/]` trapezoids
- `A@{ shape: …, label: "…" }`, with the shape names `rect`, `rounded`, `stadium`/`pill`, `subroutine`/`fr-rect`, `cyl`/`database`/`db`, `circle`/`circ`, `dbl-circ`/`double-circle`, `diam`/`diamond`/`decision`/`question`, `hex`/`hexagon`/`prepare`, `lean-r`/`in-out`, `lean-l`/`out-in`, `trap-b`, `trap-t`, `text` (no border), `odd`, `start`/`sm-circ` (a dot), `stop`/`fr-circ` (a ringed dot). Any other shape name gives a rectangle.
- Edges:
  - `-->` arrow
  - `---` line
  - `-.->` and `-.-` dotted
  - `==>` and `===` thick
  - `~~~` invisible: it affects the layout but isn't drawn
- Edge heads on either end: `>`/`<` arrow, `o` circle, `x` cross (`<-->`, `o--o`, `x--x`, `A --o B`, `A --x B`).
- Longer edges (`--->`, `-..->`, `===>`) keep the two nodes more ranks apart.
- Edge labels: `A -->|text| B`, `A -- text --> B`, `A -. text .-> B`, `A == text ==> B`.
- Chains and groups: `A --> B --> C`, `A & B --> C & D`.
- Edge ids (`A e1@--> B`) are accepted. A statement that only sets edge properties, such as `e1@{ animate: true }`, is ignored rather than read as a node. This applies when the id belongs to an edge, or when the only keys are `animate`, `animation` or `curve`.
- `subgraph id [title]`, `subgraph id[title]`, `subgraph "title"` and `subgraph some title`, each closed by `end`. Subgraphs can nest.
- An edge to a subgraph's id connects to the subgraph's frame. A node first used at the top level and then mentioned inside a subgraph moves into that subgraph.
- `direction` sets the diagram's direction at the top level. Inside a subgraph it applies only when no edge crosses the subgraph's frame, which is how Mermaid behaves too.
- Colours: `classDef a,b fill:…,stroke:…`, `class A,B a`, `A:::a`, `style A stroke:…`. A class can be assigned before its node appears. `style` overrides classes. See [Colours](#colours).

Ignored / not drawn:

- `linkStyle`, `click`, `callback`, `interpolate`, `accTitle`, `accDescr`
- Edge properties (`e1@{ animate: true }`, `curve`, …)
- `icon`, `img` and other `@{ … }` keys besides `shape` and `label`

## State diagram

Headers: `stateDiagram`, `stateDiagram-v2`.

- States are rounded boxes. A state is created by naming it on its own line or in a transition.
- `A --> B` and `A --> B : label`.
- `[*]` is the start (a dot) when it is the source of a transition and the end (a ringed dot) when it is the target. Each composite state has its own start and end.
- `state "Description" as A`.
- `A : description`. Several of these add several lines.
- Composite states: `state A {` … `}` and `state "Title" as A {` … `}`. They are drawn as frames and can nest.
- `state X <<choice>>` (a decision), `state X <<fork>>` and `state X <<join>>` (bars).
- `note left of A : text` and `note right of A : text`, or as a block that ends with `end note`. Notes are dashed boxes joined to their state by a dotted line.
- `direction` at the top level or inside a composite state.

Ignored / not drawn:

- `--` concurrency separators: the regions are not divided
- `classDef`, `class`, `hide`, `scale`, `accTitle`, `accDescr`

## Class diagram

Headers: `classDiagram`, `classDiagram-v2`.

- `class A`, `class A { … }`, and a bare `A` on its own line.
- `class A["Label"]` shows `Label` instead of the id.
- Generics: `class List~T~` and `~T~` in members are shown as `<T>`.
- Members inside braces or as `A : member`. A member with `(` goes in the methods compartment and any other member goes in the attributes compartment. Trailing `$` (static) and `*` (abstract) markers are removed.
- Annotations: `<<interface>> A`, or `<<interface>>` inside the class body. They are shown as `«interface»` above the name.
- Relations:
  - `<|--` and `--|>` inheritance (hollow triangle)
  - `*--` and `--*` composition (filled diamond)
  - `o--` and `--o` aggregation (hollow diamond)
  - `-->` and `<--` association
  - `--` link
  - `..|>`, `<|..`, `..>`, `<..`, `*..`, `o..` and `..` are the dotted forms
  - `()--` and `--()` lollipop (circle)
- Cardinalities are written next to each end: `A "1" --> "*" B`.
- A relation label goes after the colon: `A --> B : uses`.
- `note "text"` makes a free-standing note. `note for A "text"` joins the note to a class with a dotted line.
- `namespace N {` … `}` draws a frame.
- `direction`.

Ignored / not drawn:

- `classDef`, `cssClass`, `style`, `:::name` on a class declaration
- `click`, `link`, `callback`, `accTitle`, `accDescr`

## Entity relationship diagram

Header: `erDiagram`.

- `A ||--o{ B : label`. The label after the colon is required and its quotes are removed.
- End markers are written as cardinalities: `||` is `1`, `|o`/`o|` is `0..1`, `}o`/`o{` is `0..*` and `}|`/`|{` is `1..*`.
- `--` (identifying) is a solid line and `..` (non-identifying) is dotted.
- Entity names may be quoted. `A["Label"]` shows `Label` instead of the name.
- Attributes inside `A { … }`: `type name`, optionally followed by keys (`PK`, `FK`, `UK`, comma-separated) and a `"comment"`. They are shown in aligned columns.
- `direction`.

Ignored / not drawn:

- Any other line, including `style`, `classDef`, `class` and `accTitle`
- Attribute lines with only one word

## C4

Headers: `C4Context`, `C4Container`, `C4Component`, `C4Dynamic`, `C4Deployment`.

- `title …`.
- Elements are written as `Kind(alias, "label", …)`. `Person`, `System`, `Container`, `Component` and their `_Ext`, `Db` and `Queue` variants are boxes showing the label, a `[Kind]` tag and the description. For containers and components, the technology is added to the tag (`[Container: Go]`).
  - `_Ext` elements have a dashed border.
  - `…Db` elements are cylinders.
  - `…Queue` and `Person` elements are rounded.
- Boundaries: any call whose name ends in `Boundary` (`Boundary`, `System_Boundary`, `Container_Boundary`, `Enterprise_Boundary`, …) followed by `{` … `}`. They are drawn as frames titled with the label, plus `[type]` when a third argument is given.
- Deployment nodes: `Deployment_Node`, `Node` and their `_L`/`_R` variants, written as `(alias, "label", "type", …)` followed by `{` … `}`. They are frames like boundaries, titled with the label and `[type]`, and they nest.
- Any other call followed by `{` has its matching `}` skipped, so the enclosing frame isn't closed early.
- `Rel…(from, to, "label", "technology")` draws an arrow with the label, and the technology as `[technology]`. All `Rel_*` direction variants behave the same. `BiRel` puts arrows at both ends. `Rel_Back` reverses the direction.

Ignored / not drawn:

- `UpdateElementStyle`, `UpdateRelStyle`, `UpdateBoundaryStyle`, `UpdateLayoutConfig` and any other `Update…` call
- `AddElementTag`, `AddRelTag`
- Named arguments such as `$tags="…"`, `$link="…"` and `$sprite="…"`
- Any line that isn't a call

## Requirement diagram

Header: `requirementDiagram`.

- Blocks: `requirement`, `functionalRequirement`, `interfaceRequirement`, `performanceRequirement`, `physicalRequirement`, `designConstraint` and `element`, each written as `name {` … `}`. The type is shown as `«Requirement»` and so on above the name.
- Fields are `key: value`, shown below the name. `id`, `text`, `risk`, `verifymethod`, `type` and `docref` get readable names (`Verification`, `Doc ref`, …). Other keys are shown as written. Quotes around values are removed.
- Relationships: `a - satisfies -> b` and `b <- satisfies - a`, with any relationship word. They are drawn as an arrow labelled `«satisfies»`.
- `direction`.

Ignored / not drawn:

- Any other line, including `style`, `classDef`, `class` and `:::name`

## Architecture

Headers: `architecture-beta`, `architecture`.

- `group id(icon)[Title]` and `group id[Title] in parent`. Groups are drawn as frames.
- `service id(icon)[Title]` and `service … in group`. The icon name is shown above the title. `database` services are drawn as cylinders without the icon name. `internet` and `cloud` services are rounded.
- `junction id`, optionally `in group`, is drawn as a dot.
- Edges: `a:R -- L:b`, `a:R --> L:b`, `a:R <-- L:b` and `a:R <--> L:b`. The `{group}` modifier is accepted.
- The diagram is laid out left to right when more edges join left and right sides than top and bottom sides, and top to bottom otherwise.

Ignored / not drawn:

- Group icons
- Which side an edge leaves from (it only counts toward the overall direction)
- Lines in any other form

## Sequence diagram

Header: `sequenceDiagram`.

- `participant A`, `participant A as Label`, `actor A`. Actors are drawn as rounded boxes. Participants that aren't declared are created the first time they are used.
- `title …` or `title: …`.
- Messages, written as `A->>B: text`:
  - `->` solid line, no head
  - `-->` dotted line, no head
  - `->>` solid with an arrow
  - `-->>` dotted with an arrow
  - `-x` and `--x` cross
  - `-)` and `--)` arrow
  - `<<->>` and `<<-->>` arrows at both ends
- A message from a participant to itself loops out to the right.
- Activation: `+` and `-` after the arrow (`A->>+B`), or `activate A` and `deactivate A`.
- `autonumber` numbers the following messages ①, ②, … up to ⑳, then (21), (22), ….
- `note left of A: text`, `note right of A: text`, `note over A: text`, `note over A,B: text`.
- Blocks: `loop`, `alt`/`else`, `opt`, `par`/`and`, `critical`/`option` and `break`, each closed by `end`. They are drawn as frames with their label. Blocks can nest.
- `rect …` … `end` draws a frame without a label. Its colour is ignored.
- `box [colour] Title` … `end` groups participants in a dotted frame. A leading colour name, `#…` or `rgb(…)` is removed from the title.
- `create participant A` and `create actor A` make the participant appear at that point.
- A trailing `;` on a line is ignored.

Ignored / not drawn:

- `destroy` (the participant's lifeline continues)
- `link`, `links`, `accTitle`, `accDescr`
- `box` and `rect` colours

## Block diagram

Headers: `block-beta`, `block`.

- `columns N` sets the number of columns. Without it, all items go on one row.
- Items are separated by spaces: `a`, `a["text"]`, `a:2` (spans two columns).
- Shapes: `a(text)`, `a((text))`, `a([text])`, `a[(text)]`, `a[[text]]`, `a{text}`, `a{{text}}`, `a>text]`.
- `space` and `space:N` leave empty cells.
- Nested blocks: `block`, `block:id`, `block:id:2` and `block:id["Title"]:2`, each closed by `end`. A nested block can have its own `columns`.
- Edges, one per line: `a --> b`, `a --- b`, `a ==> b` (thick), `a -.-> b` (dotted), and `a -- "label" --> b`. Every edge is drawn with an arrowhead.

Ignored / not drawn:

- `style`, `classDef`, `class`
- Block arrows (`id<["text"]>(right)`)
- Width fitting: the diagram is drawn at its natural size

## Gantt

Header: `gantt`.

- `title …`, `section …`.
- `dateFormat …` using the tokens `YYYY`, `YY`, `MM`, `M`, `DD`, `D`, `Do`, `HH`, `H`, `mm`, `ss` and `SSS`, or `X` (Unix seconds) and `x` (Unix milliseconds). If a date doesn't match the format, boxmaid also tries `YYYY-MM-DD`, `YYYY-MM-DD HH:mm` and `YYYY-MM-DDTHH:mm:ss`.
- Tasks are written as `Name : tags, …`:
  - One item is the end; the task starts when the previous task ends.
  - Two items are the start and the end.
  - Three items are an id, the start and the end.
- A start is a date or `after id1 id2` (the latest end of the tasks named).
- An end is a date, a duration (`3d`, `1w`, `12h`, `30m`, `10s`, `100ms`, `2M`, `1y`, with decimals allowed) or `until id` (that task's start).
- Tags: `done` (muted bar), `active` (accent), `crit` (warning colour) and `milestone` (drawn as ◆). A task that ends where it starts is also drawn as ◆.
- `excludes weekends` stretches day and week durations over weekends.
- The date axis switches between times, days and months depending on the span of the chart.
- A task whose start or end can't be worked out is shown as `?`.

Ignored / not drawn:

- `axisFormat`, `tickInterval`, `todayMarker`, `includes`, `weekday`, `inclusiveEndDates`, `topAxis`, `displayMode`
- `click`, `accTitle`, `accDescr`
- Excluded dates other than weekends

## User journey

Header: `journey`.

- `title …`, `section …`.
- Tasks are written as `Task name: score` or `Task name: score: Actor1, Actor2`. The score is limited to 0 to 5 and drawn as dots coloured from bad to good. The actors are printed as written.

Ignored / not drawn:

- `accTitle`, `accDescr`

## Timeline

Header: `timeline`.

- `title …`, `section …`.
- `Period : event : event`.
- A line that starts with `: event` adds more events to the period above.
- Events wrap to the width.

Ignored / not drawn:

- `accTitle`, `accDescr`

## Mindmap

Header: `mindmap`.

- Indentation gives the tree. The first node is the root and is drawn in a box. The other nodes are drawn as a tree below it.
- Node shapes are accepted and only their text is shown: `id[text]`, `id(text)`, `id((text))`, `id))text((`, `id)text(`, `id{{text}}`.
- Text wraps to the width.

Ignored / not drawn:

- `::icon(…)` lines
- `:::class` suffixes
- The node shapes themselves

## Git graph

Header: `gitGraph`, optionally followed by `LR:`, `TB:` or `BT:`.

- `commit`, with the options `id: "…"`, `tag: "…"` and `type: NORMAL | HIGHLIGHT | REVERSE`.
- Commits are drawn as ● (normal), ■ (highlight), ⊗ (reverse), ◉ (merge) and ◎ (cherry-pick).
- A commit's tag is written next to it. A commit without a tag shows its id, if it has one.
- `branch name`, optionally with `order: N`. The new branch starts at the current branch's last commit and becomes the current branch. A branch that already exists is skipped.
- `checkout name` and `switch name`.
- `merge name`, with the options `id:`, `tag:` and `type:`. A merge from an unknown branch, or from a branch without commits, is skipped.
- `cherry-pick id: "…"` adds a commit tagged `← id`.
- `main` always exists and is the first lane. Each lane has its own colour from a fixed palette.

Ignored / not drawn:

- `msg:` on commits
- `parent:` on cherry-picks
- Theme and branch colour settings in `%%{init}%%`

## Pie

Header: `pie`, optionally followed by `showData`.

- `title …`, on its own line or on the header line (`pie title Pets`).
- Slices are written as `"Label" : value`. The label must be quoted. Values of zero or less are dropped.
- Slices are drawn as horizontal bars, largest first, with their percentage. `showData` also shows the value.

Ignored / not drawn:

- `accTitle`, `accDescr`

## Quadrant chart

Header: `quadrantChart`.

- `title …`.
- `x-axis Low --> High` and `y-axis Low --> High`. The labels are written at the ends of the axes.
- `quadrant-1` to `quadrant-4`, numbered as in Mermaid: 1 is top right, 2 top left, 3 bottom left and 4 bottom right.
- Points are written as `Name: [x, y]`, with x and y limited to 0 to 1.

Ignored / not drawn:

- Point styling after the coordinates (`radius`, `color`, `stroke-…`)
- `classDef`

## XY chart

Headers: `xychart-beta`, `xychart`. Add `horizontal` to the header line for sideways bars.

- `title "…"`.
- `x-axis [a, b, c]` and `x-axis "Title" [a, b, c]` set the categories. With `x-axis lo --> hi`, or without an x-axis, categories are numbered 1, 2, ….
- `y-axis "Title" lo --> hi` sets the value range. Without it, the range runs from the smallest value, or 0, to the largest. Its title is written above the value axis.
- The x-axis title is written centred under the category labels.
- `bar [values]` and `line [values]`. Several series are allowed. Bars of different series stand side by side, and line series are points joined by dots. A series name (`bar "Sales" [...]`) is accepted.
- Horizontal charts give each category a row per bar series, with its value at the end. Line series are points joined across the categories. The x-axis title heads the category names, and the y-axis title is centred under the bars.

Ignored / not drawn:

- Series names

## Kanban

Header: `kanban`.

- Lines at the lowest indentation are columns, written as `id[Title]` or `Title`. More indented lines are cards in the column above, written as `id[text]`.
- Card metadata `@{ … }`: `assigned` is shown as `@name` and `priority` is coloured (`Very High` and `High` as a warning, `Low` and `Very Low` as good, anything else as accent). Other keys, such as `ticket`, are shown as their value.
- Each column shows its card count. Columns that don't fit the width wrap onto a new row.

Ignored / not drawn:

- Label handling (entities, `<br>`) in column titles. Card text is handled as a label.

## Packet

Headers: `packet-beta`, `packet`.

- `title …`.
- Fields are written as `0-15: "Name"`, `16: "Name"` (one bit) or `+8: "Name"` (the next 8 bits).
- Fields are drawn as boxes on rows of 32 bits, with the bit numbers above each row.

## Sankey

Headers: `sankey-beta`, `sankey`.

- CSV rows: `source,target,value`. Fields can be quoted with `"…"`, and `""` inside quotes stands for a quote.
- Each source is listed with its outflows as bars proportional to their value, sources first, in flow order.

Ignored / not drawn:

- Rows without at least three fields or with a value that isn't a number (such as a CSV header row)

## Radar

Headers: `radar-beta`, `radar`.

- `title …`.
- `axis a["Label"], b, …`.
- `curve id["Label"]{1, 2, 3}` gives the values in axis order, and `curve id{a: 1, b: 2}` gives them by axis id.
- `max N` sets the scale. Without it, the largest value is used.
- For each axis, every curve's value is shown as a bar.

Ignored / not drawn:

- `min`, `ticks`, `graticule`, `showLegend`

## Treemap

Headers: `treemap-beta`, `treemap`.

- `title …`.
- Indentation gives the hierarchy. A leaf is written as `"Name": value`. A parent's value is the sum of its children.
- Nodes are drawn as nested boxes whose areas follow the values, split alternately across and down. Each depth has its own colour.

Ignored / not drawn:

- `classDef` lines and `:::class` suffixes
