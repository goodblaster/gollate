//go:build darwin

#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <CoreImage/CoreImage.h>

const char *performAppleVisionOCR(const void *imageBytes, size_t length, const char **langs, size_t langsCount, int recognitionLevel) {
    @autoreleasepool {
        // Create NSData from the provided image bytes
        NSData *imageData = [NSData dataWithBytes:imageBytes length:length];
        if (!imageData) {
            return strdup("{\"error\": \"Failed to create NSData from image bytes.\"}");
        }

        // Create a CIImage from the NSData
        CIImage *ciImage = [CIImage imageWithData:imageData];
        if (!ciImage) {
            return strdup("{\"error\": \"Failed to create CIImage from NSData.\"}");
        }

        // Create a request handler
        VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCIImage:ciImage options:@{}];

        // Create an array for languages
        NSMutableArray *languages = [NSMutableArray array];

        // If an array of languages is provided, add them first (in priority order)
        if (langs != NULL && langsCount > 0) {
            for (size_t i = 0; i < langsCount; i++) {
                NSString *specifiedLanguage = [NSString stringWithUTF8String:langs[i]];
                if (specifiedLanguage) {
                    [languages addObject:specifiedLanguage];
                }
            }
        }

        // Add en-US as fallback if it wasn't already specified
        BOOL hasEnglish = NO;
        for (NSString *lang in languages) {
            if ([lang isEqualToString:@"en-US"]) {
                hasEnglish = YES;
                break;
            }
        }
        if (!hasEnglish) {
            [languages addObject:@"en-US"];
        }

        // Set up the OCR request using the languages array
        VNRecognizeTextRequest *request = [[VNRecognizeTextRequest alloc] init];
        // recognitionLevel: 0 = fast, 1 = accurate
        // Fast is much faster on large images and works well for Latin text
        // Accurate is needed for better CJK character recognition
        request.recognitionLevel = (recognitionLevel == 1) ? VNRequestTextRecognitionLevelAccurate : VNRequestTextRecognitionLevelFast;
        request.recognitionLanguages = languages;

        NSError *error = nil;
        [handler performRequests:@[request] error:&error];
        if (error) {
            NSString *errorJSON = [NSString stringWithFormat:@"{\"error\": \"%@\"}", error.localizedDescription];
            return strdup([errorJSON UTF8String]);
        }

        // Collect recognized text with granular coordinates
        NSMutableArray *results = [NSMutableArray array];
        for (VNRecognizedTextObservation *observation in request.results) {
            NSArray<VNRecognizedText *> *topCandidates = [observation topCandidates:1];
            if (topCandidates.count > 0) {
                VNRecognizedText *text = topCandidates.firstObject;

                // Split text into words
                // For mixed content: split CJK characters individually, keep ASCII together
                // Examples: "1857年" -> ["1857", "年"], "維基百科" -> ["維", "基", "百", "科"]
                NSArray *rawWords = [text.string componentsSeparatedByCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
                NSMutableArray *wordsArray = [NSMutableArray array];
                for (NSString *word in rawWords) {
                    if (word.length == 0) continue;

                    // Check if word contains any CJK characters
                    BOOL hasCJK = NO;
                    for (NSInteger i = 0; i < word.length; i++) {
                        unichar c = [word characterAtIndex:i];
                        if ((c >= 0x3400 && c <= 0x4DBF) ||  // CJK Extension A
                            (c >= 0x4E00 && c <= 0x9FFF) ||  // CJK Unified Ideographs
                            (c >= 0x3040 && c <= 0x309F) ||  // Hiragana
                            (c >= 0x30A0 && c <= 0x30FF)) {  // Katakana
                            hasCJK = YES;
                            break;
                        }
                    }

                    // If no CJK characters, keep word as-is (preserves English and phone numbers)
                    if (!hasCJK) {
                        [wordsArray addObject:word];
                        continue;
                    }

                    // For mixed CJK/ASCII: split into tokens
                    // "1857年" -> ["1857", "年"]
                    NSMutableString *currentToken = [NSMutableString string];
                    BOOL lastWasCJK = NO;

                    for (NSInteger i = 0; i < word.length; i++) {
                        unichar c = [word characterAtIndex:i];

                        BOOL isCJK = (c >= 0x3400 && c <= 0x4DBF) ||
                                     (c >= 0x4E00 && c <= 0x9FFF) ||
                                     (c >= 0x3040 && c <= 0x309F) ||
                                     (c >= 0x30A0 && c <= 0x30FF);

                        // If switching between CJK and non-CJK, flush buffer
                        if (currentToken.length > 0 && isCJK != lastWasCJK) {
                            [wordsArray addObject:[currentToken copy]];
                            [currentToken setString:@""];
                        }

                        if (isCJK) {
                            // CJK characters are added individually
                            if (currentToken.length > 0) {
                                [wordsArray addObject:[currentToken copy]];
                                [currentToken setString:@""];
                            }
                            [wordsArray addObject:[NSString stringWithFormat:@"%C", c]];
                        } else {
                            // Accumulate ASCII/numbers together
                            [currentToken appendFormat:@"%C", c];
                        }

                        lastWasCJK = isCJK;
                    }

                    // Flush remaining token
                    if (currentToken.length > 0) {
                        [wordsArray addObject:[currentToken copy]];
                    }
                }

                // Calculate total characters in words (excluding spaces)
                NSInteger totalWordChars = 0;
                for (NSString *word in wordsArray) {
                    totalWordChars += word.length;
                }

                // Use proportional widths based on character count of each word
                NSMutableArray *words = [NSMutableArray array];
                CGFloat currentX = observation.boundingBox.origin.x;
                for (NSString *word in wordsArray) {
                    CGFloat wordFraction = (totalWordChars > 0) ? ((CGFloat)word.length / totalWordChars) : 0;
                    CGFloat wordWidth = observation.boundingBox.size.width * wordFraction;
                    CGRect wordBox = CGRectMake(
                        currentX,
                        observation.boundingBox.origin.y,
                        wordWidth,
                        observation.boundingBox.size.height
                    );
                    currentX += wordWidth; // Move to the next word position

                    NSDictionary *wordInfo = @{
                        @"text": word,
                        @"left": @(wordBox.origin.x),
                        @"top": @(1.0 - wordBox.origin.y - wordBox.size.height),
                        @"width": @(wordBox.size.width),
                        @"height": @(wordBox.size.height)
                    };

                    [words addObject:wordInfo];
                }

                // Include line bounding box information
                NSDictionary *lineDimensions = @{
                    @"left": @(observation.boundingBox.origin.x),
                    @"top": @(1.0 - observation.boundingBox.origin.y - observation.boundingBox.size.height),
                    @"width": @(observation.boundingBox.size.width),
                    @"height": @(observation.boundingBox.size.height)
                };

                // Add result for this observation
                NSDictionary *lineInfo = @{
                    @"text": text.string,
                    @"confidence": @(text.confidence),
                    @"rect": lineDimensions,
                    @"words": words
                };
                [results addObject:lineInfo];
            }
        }

        // Convert results to JSON
        NSError *jsonError = nil;
        NSData *jsonData = [NSJSONSerialization dataWithJSONObject:results options:NSJSONWritingPrettyPrinted error:&jsonError];
        if (jsonError) {
            return strdup("{\"error\": \"Failed to generate JSON.\"}");
        }

        return strdup([[NSString alloc] initWithData:jsonData encoding:NSUTF8StringEncoding].UTF8String);
    }
}
