# OCR Sorting Algorithm

This document provides a visual explanation of the gollate algorithm using flowcharts and diagrams.

> **2026-07 update:** the flowcharts below describe the core pathfinding
> pipeline. Since they were drawn, four error-tolerance mechanisms were
> added inside the pathfinding stage — wrap bridging (paths may cross
> visual line wraps), wildcard holes (words missing from OCR are bridged
> instead of splitting the line), short-line anchoring (duplicated short
> lines pick the instance nearest their matched canonical neighbors), and
> a reconciliation pass (anchor-gated rescue of unfound fragments after
> the main loop). They don't change the overall flow shown here; see
> pkg/sorters/README.md ("Error-tolerance mechanisms") for how they slot in.

## High-Level Algorithm Flow

```mermaid
flowchart TD
    Start([Start: OCR Blocks + Canonical Text]) --> Normalize[Normalize Text & Blocks<br/>- Remove punctuation<br/>- Lowercase<br/>- Handle Unicode]

    Normalize --> SortLines[Sort Canonical Lines<br/>by Length<br/>Longest First]

    SortLines --> BuildMapping[Build Block Mapping<br/>word to block indices]

    BuildMapping --> PassLoop{Pass Loop<br/>max passes?}

    PassLoop -->|No| ProcessLines[Process Lines<br/>in Current Pass]

    ProcessLines --> LineLoop{For Each<br/>Line}

    LineLoop -->|Next Line| FindPaths[Find All Possible Paths<br/>through OCR blocks]

    FindPaths --> HasPaths{Paths<br/>Found?}

    HasPaths -->|Yes| SelectShortest[Select Shortest Path<br/>by Spatial Distance]

    SelectShortest --> RemoveBlocks[Remove Used Blocks<br/>from Mapping]

    RemoveBlocks --> LineLoop

    HasPaths -->|No| TrySplit{Can Split<br/>Line?}

    TrySplit -->|Yes| SplitLine[Split Line by<br/>- Missing words<br/>- Sentence boundaries]

    SplitLine --> LineLoop

    TrySplit -->|No| MarkUnhandled[Mark Line as<br/>Unhandled]

    MarkUnhandled --> LineLoop

    LineLoop -->|Done| PassLoop

    PassLoop -->|Yes| AddLeftovers[Add Leftover Blocks<br/>in Spatial Order]

    AddLeftovers --> InsertBreaks[Insert Line Breaks<br/>Between Sentences]

    InsertBreaks --> End([Return Sorted Blocks])

    style Start fill:#e1f5e1
    style End fill:#e1f5e1
    style FindPaths fill:#fff3cd
    style SelectShortest fill:#cfe2ff
    style BuildMapping fill:#f8d7da
```

## Pathfinding Algorithm (Detailed)

```mermaid
flowchart TD
    Start([Input: Canonical Line Words]) --> InitPath[Initialize Empty Path<br/>Distance = 0]

    InitPath --> WordLoop{For Each<br/>Word in Line}

    WordLoop -->|Next Word| ExactMatch{Exact Match<br/>in Mapping?}

    ExactMatch -->|Yes| GetCandidates[Get Candidate Blocks<br/>for This Word]

    ExactMatch -->|No| AbsentWord[Mark Word as Absent<br/>Line Cannot Be Matched]

    AbsentWord --> Return([Return: No Valid Path])

    GetCandidates --> RotationOpt{Rotation<br/>Optimization?}

    RotationOpt -->|Yes| SortByDistance[Sort Candidates by<br/>Distance from Previous Block]

    RotationOpt -->|No| KeepOrder[Keep Original Order]

    SortByDistance --> TryBlock
    KeepOrder --> TryBlock

    TryBlock{Try Each<br/>Candidate Block}

    TryBlock -->|Next Block| AlreadyUsed{Block Already<br/>in Path?}

    AlreadyUsed -->|Yes| TryBlock

    AlreadyUsed -->|No| CalcDistance[Calculate Distance<br/>from Previous Block]

    CalcDistance --> TooFar{Distance ><br/>Max Threshold?}

    TooFar -->|Yes| TryBlock

    TooFar -->|No| ExtendPath[Extend Path with Block<br/>Update Total Distance]

    ExtendPath --> MoreWords{More Words<br/>in Line?}

    MoreWords -->|Yes| Recurse[Recursive Call<br/>for Next Word]

    Recurse --> RecurseResult{Path Found<br/>for Remaining?}

    RecurseResult -->|Yes| RecordPath[Record Complete Path<br/>Update Shortest if Better]

    RecurseResult -->|No| Backtrack[Backtrack:<br/>Remove Block from Path]

    Backtrack --> TryBlock

    RecordPath --> TryBlock

    MoreWords -->|No| RecordPath

    TryBlock -->|No More| ReturnBest([Return: Best Path Found])

    WordLoop -->|Done| ReturnBest

    style Start fill:#e1f5e1
    style Return fill:#f8d7da
    style ReturnBest fill:#e1f5e1
    style CalcDistance fill:#cfe2ff
```

## Distance Calculation (Reading Order Aware)

```mermaid
flowchart LR
    subgraph "Horizontal LTR (English, Spanish, French)"
        H1[Block A] -->|Primary: Horizontal Δ| H2[Block B]
        H2 -->|Secondary: Vertical Δ| H3[Distance]
        H3 --> H4[Prefer: Right & Down]
    end

    subgraph "Horizontal RTL (Arabic, Hebrew)"
        R1[Block A] -->|Primary: Horizontal Δ| R2[Block B]
        R2 -->|Secondary: Vertical Δ| R3[Distance]
        R3 --> R4[Prefer: Left & Down]
    end

    subgraph "Vertical TTB RTL (Chinese, Japanese)"
        V1[Block A] -->|Primary: Vertical Δ| V2[Block B]
        V2 -->|Secondary: Horizontal Δ| V3[Distance]
        V3 --> V4[Prefer: Down & Left]
    end

    style H4 fill:#d4edda
    style R4 fill:#fff3cd
    style V4 fill:#cfe2ff
```

## Block Mapping Structure

```mermaid
graph TD
    subgraph "Canonical Text"
        C1["Line 1: Lorem Ipsum is simply dummy text"]
        C2["Line 2: of the printing industry"]
    end

    subgraph "OCR Blocks (Unordered)"
        B0["[0] Lorem<br/>top:0.1, left:0.2"]
        B1["[1] Ipsum<br/>top:0.1, left:0.3"]
        B2["[2] is<br/>top:0.1, left:0.4"]
        B3["[3] simply<br/>top:0.15, left:0.2"]
        B4["[4] dummy<br/>top:0.15, left:0.3"]
        B5["[5] text<br/>top:0.15, left:0.4"]
        B6["[6] of<br/>top:0.2, left:0.2"]
        B7["[7] the<br/>top:0.2, left:0.25"]
        B8["[8] printing<br/>top:0.2, left:0.3"]
        B9["[9] industry<br/>top:0.2, left:0.4"]
    end

    subgraph "Mapping (Word → Block Indices)"
        M1["'lorem' → [0]"]
        M2["'ipsum' → [1]"]
        M3["'is' → [2]"]
        M4["'simply' → [3]"]
        M5["'dummy' → [4]"]
        M6["'text' → [5]"]
        M7["'of' → [6]"]
        M8["'the' → [7]"]
        M9["'printing' → [8]"]
        M10["'industry' → [9]"]
    end

    C1 --> M1
    C1 --> M2
    C1 --> M3
    C1 --> M4
    C1 --> M5
    C1 --> M6

    C2 --> M7
    C2 --> M8
    C2 --> M9
    C2 --> M10

    M1 --> B0
    M2 --> B1
    M3 --> B2
    M4 --> B3
    M5 --> B4
    M6 --> B5
    M7 --> B6
    M8 --> B7
    M9 --> B8
    M10 --> B9

    style C1 fill:#e1f5e1
    style C2 fill:#e1f5e1
    style M1 fill:#fff3cd
    style M2 fill:#fff3cd
    style M3 fill:#fff3cd
    style M4 fill:#fff3cd
    style M5 fill:#fff3cd
    style M6 fill:#fff3cd
    style M7 fill:#fff3cd
    style M8 fill:#fff3cd
    style M9 fill:#fff3cd
    style M10 fill:#fff3cd
```

## Path Selection Example

```mermaid
graph TD
    Start[Canonical: 'lorem ipsum is'] --> FindPaths[Find All Possible Paths]

    FindPaths --> P1["Path 1: [0,1,2]<br/>Distance: 0.15"]
    FindPaths --> P2["Path 2: [0,1,2] (different blocks)<br/>Distance: 0.45"]
    FindPaths --> P3["Path 3: [0,1,2] (wrapped)<br/>Distance: 0.30"]

    P1 --> Shortest{Select Shortest<br/>Distance Path}
    P2 --> Shortest
    P3 --> Shortest

    Shortest --> Winner["✓ Path 1 Selected<br/>Blocks: [0,1,2]<br/>Reason: Smallest spatial distance"]

    Winner --> Remove[Remove blocks 0,1,2<br/>from mapping]

    Remove --> Next[Process Next Line]

    style Start fill:#e1f5e1
    style P1 fill:#d4edda
    style P2 fill:#f8d7da
    style P3 fill:#f8d7da
    style Winner fill:#d4edda
```

## Multi-Pass Strategy

```mermaid
gantt
    title Multi-Pass Line Processing Strategy
    dateFormat X
    axisFormat %s

    section Pass 1
    Long lines (16+ words)     :p1, 0, 3

    section Pass 2
    Medium lines (10-15 words) :p2, 3, 5

    section Pass 3
    Short lines (5-9 words)    :p3, 5, 7

    section Pass 4
    Very short lines (1-4)     :p4, 7, 8

    section Pass 5
    Split unmatched lines      :p5, 8, 10

    section Pass 6
    Retry with splits          :p6, 10, 11

    section Cleanup
    Add leftover blocks        :p7, 11, 12
    Insert line breaks         :p8, 12, 13
```

## Optimization Strategies

```mermaid
mindmap
  root((OCR Sort<br/>Optimizations))
    Precurse
      Analyze first N words
      Find best starting block
      Reduces search space
    Rotation
      Sort candidates by distance
      Try nearest blocks first
      Prunes bad paths early
    Permutation Limits
      Max total permutations
      Max per-pass permutations
      Prevents exponential blowup
    Line Splitting
      Split by absent words
      Split by punctuation
      Enables partial matching
    Block Reuse Prevention
      Remove used blocks
      Each block used once
      Ensures correctness
```

## Key Algorithm Insights

### 1. Why Longest-First?

Longer lines have more words, making them more distinctive and easier to match accurately. By finding these first, we:
- Reduce the search space for shorter lines
- Establish spatial anchors in the document
- Improve overall accuracy

### 2. Why Shortest Path?

Words that appear sequentially in canonical text should appear spatially close in the document. The shortest path represents the most natural reading order.

### 3. Why Multiple Passes?

Some lines cannot be matched in early passes due to:
- Missing words (OCR failures)
- Too many permutations (combinatorial explosion)
- Ambiguous matches (multiple valid paths)

Later passes handle these with:
- Line splitting
- Reduced permutation limits
- Different word length thresholds

### 4. Why Remove Used Blocks?

Each OCR block should appear exactly once in the output. Removing used blocks:
- Prevents duplicate text in results
- Reduces search space for subsequent lines
- Ensures logical consistency

## Performance Characteristics

```mermaid
graph LR
    subgraph "Best Case"
        B1[Clean OCR] --> B2[Distinctive Text]
        B2 --> B3[Linear Layout]
        B3 --> B4["O(n log n)"]
    end

    subgraph "Average Case"
        A1[Good OCR] --> A2[Normal Text]
        A2 --> A3[Standard Layout]
        A3 --> A4["O(n²)"]
    end

    subgraph "Worst Case"
        W1[Poor OCR] --> W2[Repetitive Text]
        W2 --> W3[Complex Layout]
        W3 --> W4["O(n³) with limits"]
    end

    style B4 fill:#d4edda
    style A4 fill:#fff3cd
    style W4 fill:#f8d7da
```

## Configuration Impact

```mermaid
graph TD
    subgraph "Fast Config"
        F1[Low MaxPermutations] --> F2[Short PrecurseLength]
        F2 --> F4[Result: Fast but may miss matches]
    end

    subgraph "Accurate Config"
        A1[High MaxPermutations] --> A2[Long PrecurseLength]
        A2 --> A4[Result: Slow but finds more matches]
    end

    subgraph "Balanced Config (Default)"
        B1[Medium MaxPermutations<br/>500,000] --> B2[Medium PrecurseLength<br/>8 words]
        B2 --> B4[Result: Good speed & accuracy]
    end

    style F4 fill:#fff3cd
    style A4 fill:#d4edda
    style B4 fill:#cfe2ff
```

---

## References

- See [README.md](README.md) for usage examples
- See [CLAUDE.md](CLAUDE.md) for development guidelines
- See source code in `pkg/sorters/` for implementation details
