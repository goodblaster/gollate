# OCR-Sort Examples

This directory contains example programs demonstrating various use cases and features of gollate.

## Running Examples

Each example is a standalone Go program. To run an example:

```bash
cd examples/basic
go run main.go
```

## Available Examples

### 1. Basic Usage (`basic/`)

Demonstrates the simplest way to use gollate with default configuration.

**Key concepts:**
- Creating a SortRequest
- Parsing OCR data
- Running the sort
- Getting sorted output
- Viewing metrics

**Run:**
```bash
cd examples/basic && go run main.go
```

### 2. Custom Configuration (`custom-config/`)

Shows how to customize algorithm parameters for specific needs.

**Key concepts:**
- Creating custom SorterConfig
- Validating configuration
- Tuning parameters for performance
- Understanding configuration options

**When to use:**
- Complex documents need more permutations
- Performance optimization required
- Specific document type handling

**Run:**
```bash
cd examples/custom-config && go run main.go
```

### 3. Newspaper Columns (`newspaper-columns/`)

Demonstrates handling multi-column layouts like newspapers and magazines.

**Key concepts:**
- Multi-column reading order
- Spatial proximity algorithm
- Column detection

**Use cases:**
- Newspaper articles
- Magazine layouts
- Academic papers with columns
- Brochures and flyers

**Run:**
```bash
cd examples/newspaper-columns && go run main.go
```

### 4. Performance Metrics (`metrics/`)

Demonstrates collecting and analyzing detailed performance metrics.

**Key concepts:**
- Metrics collection
- Performance analysis
- Efficiency calculations
- Configuration tuning based on metrics

**Use cases:**
- Performance debugging
- Algorithm behavior understanding
- Production monitoring
- Configuration optimization

**Run:**
```bash
cd examples/metrics && go run main.go
```

### 5. Debug Output (`debug/`)

Debugging example showing internal block parsing and sorting details.

**Key concepts:**
- Block inspection
- Internal diagnostics
- Configuration debugging
- Troubleshooting failed sorts

**Run:**
```bash
cd examples/debug && go run main.go
```

## Common Patterns

### Error Handling

```go
if err := request.Parse(); err != nil {
    log.Fatalf("Failed to parse: %v", err)
}

if _, err := sorter.Sort(); err != nil {
    log.Fatalf("Sort failed: %v", err)
}
```

### Configuration Validation

```go
config := sorters.SorterConfig{...}
if err := config.Validate(); err != nil {
    log.Fatalf("Invalid config: %v", err)
}
```

### Using Metrics

```go
metrics := sorter.Metrics()
fmt.Printf("Completed in %d passes\n", metrics.PassesCompleted)
fmt.Printf("Found %d/%d lines\n", metrics.LinesFound, len(canonical Text))
```

## Integration Tips

### With Real OCR Systems

Replace the example JSON with actual OCR output:

```go
// Read from file
ocrData, err := os.ReadFile("ocr-output.json")
if err != nil {
    log.Fatal(err)
}

request := api.SortRequest{
    Engine:     "apple",  // or "tesseract", "easyocr"
    Lines:      canonicalText,
    InputJson:  string(ocrData),
    PageWidth:  imageWidth,
    PageHeight: imageHeight,
}
```

### Production Usage

For production systems, consider:

1. **Validation**: Always validate input and configuration
2. **Logging**: Use structured logging for debugging
3. **Metrics**: Monitor performance metrics
4. **Error Handling**: Handle errors gracefully
5. **Timeouts**: Set appropriate timeouts for large documents
6. **Resource Limits**: Configure MaxPermutations based on available resources

```go
config := sorters.DefaultConfig()
config.MaxPermutations = 200000  // Adjust based on your needs

if err := config.Validate(); err != nil {
    return fmt.Errorf("invalid configuration: %w", err)
}

logger := yourLogger.New()
sorter := sorters.NewOcrSorterWithConfig(blocks, text, logger, config)

// Run with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

done := make(chan error)
go func() {
    _, err := sorter.Sort()
    done <- err
}()

select {
case err := <-done:
    if err != nil {
        return fmt.Errorf("sort failed: %w", err)
    }
case <-ctx.Done():
    return fmt.Errorf("sort timeout exceeded")
}
```

## Further Reading

- [Main README](../README.md) - General overview and API documentation
- [GoDoc](https://pkg.go.dev/github.com/goodblaster/gollate) - Full API reference
- [Algorithm Details](../README.md#algorithm) - How the sorting algorithm works
