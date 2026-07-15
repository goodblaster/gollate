#import <Foundation/Foundation.h>
#import <PDFKit/PDFKit.h>

// extractPDFText extracts positioned words from a PDF's text layer using
// PDFKit. It returns a strdup'd JSON string the caller must free:
//
//   [
//     {
//       "width": 962.0,            // media box width in points
//       "height": 2112.0,          // media box height in points
//       "words": [
//         {"text": "Hello", "top": 0.1, "left": 0.2, "width": 0.05, "height": 0.01},
//         ...
//       ]
//     },
//     ...                          // one entry per page
//   ]
//
// Word coordinates are fractions of the media box (0-1) with a top-left
// origin (PDF's native bottom-left origin is flipped here), matching the
// gollate engine-neutral blocks convention. Words are emitted in the order
// PDFKit reports the page text. On failure returns {"error": "..."}.
//
// Pages with no text layer produce an empty words array; scanned PDFs
// without embedded text yield no words at all (callers should fall back
// to OCR). Page /Rotate is ignored, consistent with scripts/pdf-to-png.sh.
const char *extractPDFText(const char *path) {
    @autoreleasepool {
        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        PDFDocument *doc = [[PDFDocument alloc] initWithURL:url];
        if (!doc) {
            return strdup("{\"error\": \"Failed to open PDF document.\"}");
        }
        if (doc.isLocked) {
            return strdup("{\"error\": \"PDF document is password-protected.\"}");
        }

        NSCharacterSet *ws = [NSCharacterSet whitespaceAndNewlineCharacterSet];
        NSMutableArray *pages = [NSMutableArray array];

        for (NSUInteger p = 0; p < doc.pageCount; p++) {
            PDFPage *page = [doc pageAtIndex:p];
            NSRect media = [page boundsForBox:kPDFDisplayBoxMediaBox];
            NSMutableArray *words = [NSMutableArray array];

            NSString *text = page.string;
            NSUInteger len = text.length;
            NSUInteger i = 0;
            while (i < len && media.size.width > 0 && media.size.height > 0) {
                while (i < len && [ws characterIsMember:[text characterAtIndex:i]]) i++;
                NSUInteger start = i;
                while (i < len && ![ws characterIsMember:[text characterAtIndex:i]]) i++;
                if (i == start) continue;

                NSRange range = NSMakeRange(start, i - start);
                PDFSelection *sel = [page selectionForRange:range];
                if (!sel) continue;
                NSRect b = [sel boundsForPage:page];
                if (b.size.width <= 0 || b.size.height <= 0) continue;

                // Flip from PDF bottom-left origin to top-left fractions.
                double left = (b.origin.x - media.origin.x) / media.size.width;
                double top = (NSMaxY(media) - NSMaxY(b)) / media.size.height;
                double w = b.size.width / media.size.width;
                double h = b.size.height / media.size.height;
                if (left < 0) { w += left; left = 0; }
                if (top < 0) { h += top; top = 0; }
                if (left > 1 || top > 1 || w <= 0 || h <= 0) continue;
                if (left + w > 1) w = 1 - left;
                if (top + h > 1) h = 1 - top;

                [words addObject:@{
                    @"text": [text substringWithRange:range],
                    @"top": @(top),
                    @"left": @(left),
                    @"width": @(w),
                    @"height": @(h),
                }];
            }

            [pages addObject:@{
                @"width": @(media.size.width),
                @"height": @(media.size.height),
                @"words": words,
            }];
        }

        NSError *err = nil;
        NSData *jsonData = [NSJSONSerialization dataWithJSONObject:pages options:0 error:&err];
        if (!jsonData) {
            return strdup("{\"error\": \"Failed to generate JSON.\"}");
        }
        return strdup([[NSString alloc] initWithData:jsonData encoding:NSUTF8StringEncoding].UTF8String);
    }
}
