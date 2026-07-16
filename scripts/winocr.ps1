# winocr.ps1 - OCR an image with the Windows built-in OCR engine
# (Windows.Media.Ocr) and save gollate's engine-neutral blocks JSON.
#
# The Windows counterpart of ocr-util (Apple Vision): free, preinstalled,
# word-level bounding boxes. Runs on Windows PowerShell 5.1 (preinstalled
# on every Windows 10/11 machine) - NOT PowerShell 7 (pwsh), whose .NET
# runtime cannot project WinRT types this way.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\winocr.ps1 page.png
#   powershell ... -File scripts\winocr.ps1 page.png -Language ja
#   powershell ... -File scripts\winocr.ps1 -ListLanguages
#
# Output: {basename}-winocr.json - a blocks-format array usable directly:
#   gollate --engine blocks --ocr-file page-winocr.json --text-file canon.txt `
#           --width <px> --height <px> --language english
#
# Notes:
# - OCR languages are Windows language packs. List installed ones with
#   -ListLanguages; add more in Settings > Time & Language > Language
#   (add the language, include the "Optical character recognition"
#   feature), or:  Add-WindowsCapability -Online -Name "Language.OCR~~~ja-JP~0.0.1.0"
# - Windows.Media.Ocr rejects images larger than OcrEngine::MaxImageDimension
#   (2600 px) on either side. Standard 2x test pages (1632x2112) fit; very
#   tall pages must be sliced or scaled before OCR (no auto-slicing here yet).

param(
    [Parameter(Position = 0)]
    [string]$ImagePath,
    [string]$Language,
    [string]$OutFile,
    [switch]$ListLanguages
)

$ErrorActionPreference = 'Stop'

if ($PSVersionTable.PSEdition -eq 'Core') {
    Write-Error "Run this under Windows PowerShell 5.1 (powershell.exe), not pwsh: PowerShell 7 cannot project WinRT types."
    exit 1
}
if (-not [Environment]::OSVersion.Platform.ToString().StartsWith('Win')) {
    Write-Error "Windows.Media.Ocr requires Windows."
    exit 1
}

# Project the WinRT types into the session.
$null = [Windows.Media.Ocr.OcrEngine,Windows.Foundation,ContentType=WindowsRuntime]
$null = [Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime]
$null = [Windows.Graphics.Imaging.BitmapDecoder,Windows.Graphics,ContentType=WindowsRuntime]
$null = [Windows.Globalization.Language,Windows.Globalization,ContentType=WindowsRuntime]
Add-Type -AssemblyName System.Runtime.WindowsRuntime

# Await bridges WinRT IAsyncOperation<T> to a .NET Task we can block on.
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() |
    Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and
        $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' })[0]

function Await($WinRtTask, $ResultType) {
    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
    $netTask = $asTask.Invoke($null, @($WinRtTask))
    $null = $netTask.Wait(-1)
    $netTask.Result
}

if ($ListLanguages) {
    $langs = [Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages
    if ($langs.Count -eq 0) {
        Write-Host "No OCR languages installed. Add one in Settings > Time & Language > Language."
    } else {
        Write-Host "Installed OCR languages:"
        foreach ($l in $langs) { Write-Host ("  {0}  {1}" -f $l.LanguageTag, $l.DisplayName) }
    }
    exit 0
}

if (-not $ImagePath) {
    Write-Error "Usage: winocr.ps1 <image-file> [-Language <bcp47>] [-OutFile <path>] [-ListLanguages]"
    exit 1
}
if (-not (Test-Path $ImagePath)) {
    Write-Error "Image file not found: $ImagePath"
    exit 1
}

# Create the engine before decoding so language errors are fast and clear.
if ($Language) {
    $lang = New-Object Windows.Globalization.Language($Language)
    $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($lang)
    if (-not $engine) {
        $installed = ([Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages |
            ForEach-Object { $_.LanguageTag }) -join ', '
        Write-Error "No OCR support for language '$Language'. Installed: $installed. Add packs in Settings > Time & Language > Language."
        exit 1
    }
} else {
    $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
    if (-not $engine) {
        Write-Error "No OCR language available for the user profile. Pass -Language or install a language pack."
        exit 1
    }
}

$fullPath = (Resolve-Path $ImagePath).Path
$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($fullPath)) ([Windows.Storage.StorageFile])
$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
$decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
$bitmap = Await ($decoder.GetSoftwareBitmapAsync([Windows.Graphics.Imaging.BitmapPixelFormat]::Bgra8,
        [Windows.Graphics.Imaging.BitmapAlphaMode]::Premultiplied)) ([Windows.Graphics.Imaging.SoftwareBitmap])

$w = [double]$bitmap.PixelWidth
$h = [double]$bitmap.PixelHeight
$maxDim = [Windows.Media.Ocr.OcrEngine]::MaxImageDimension
if ($w -gt $maxDim -or $h -gt $maxDim) {
    Write-Error ("Image is {0}x{1} px; Windows OCR accepts at most {2} px per side. Slice or scale the image first." -f $w, $h, $maxDim)
    exit 1
}

$result = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])

# Convert to gollate's engine-neutral blocks format: word granularity,
# 0-1 page fractions, emit order = engine order, line_id from the
# engine's own line grouping (feeds vertical detection and line repair).
$blocks = New-Object System.Collections.Generic.List[object]
$lineNum = 0
foreach ($line in $result.Lines) {
    $lineNum++
    foreach ($word in $line.Words) {
        $r = $word.BoundingRect
        $blocks.Add([ordered]@{
                text            = $word.Text
                bounds          = [ordered]@{
                    top    = [Math]::Max(0, [Math]::Min(1, $r.Y / $h))
                    left   = [Math]::Max(0, [Math]::Min(1, $r.X / $w))
                    width  = [Math]::Min($r.Width / $w, 1)
                    height = [Math]::Min($r.Height / $h, 1)
                }
                normalized_conf = 1.0
                engine          = 'winocr'
                line_id         = "$lineNum"
            })
    }
}

if (-not $OutFile) {
    $base = [System.IO.Path]::Combine([System.IO.Path]::GetDirectoryName($fullPath),
        [System.IO.Path]::GetFileNameWithoutExtension($fullPath))
    $OutFile = "$base-winocr.json"
} elseif (-not [System.IO.Path]::IsPathRooted($OutFile)) {
    # .NET file APIs resolve relative paths against the process CWD, which
    # can differ from the PowerShell location.
    $OutFile = [System.IO.Path]::GetFullPath([System.IO.Path]::Combine((Get-Location).Path, $OutFile))
}

# Write UTF-8 WITHOUT BOM: PS 5.1's -Encoding UTF8 adds a BOM, which Go's
# JSON decoder rejects. ConvertTo-Json also emits an empty string for
# empty lists, so write [] explicitly.
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
if ($blocks.Count -eq 0) {
    [System.IO.File]::WriteAllText($OutFile, '[]', $utf8NoBom)
    Write-Warning "No text recognized (wrong language pack? blank image?)."
} else {
    $json = ConvertTo-Json -InputObject $blocks -Depth 4
    [System.IO.File]::WriteAllText($OutFile, $json, $utf8NoBom)
}

Write-Host ("Processing: {0} ({1}x{2}, engine language: {3})" -f $ImagePath, $w, $h, $engine.RecognizerLanguage.LanguageTag)
Write-Host ("Saved: {0}" -f $OutFile)
Write-Host ("  Lines: {0}" -f $result.Lines.Count)
Write-Host ("  Words: {0}" -f $blocks.Count)
Write-Host ("  (pass --width {0} --height {1} to gollate, with --engine blocks)" -f [int]$w, [int]$h)
