# Skool-Courses-Scraper: Technical Documentation

## Executive Summary

This document provides a comprehensive technical analysis of the Skool-Courses-scraper project, a Go-based utility designed to extract and archive educational content from Skool.com classrooms. The analysis covers code structure, architecture, functionality, usage patterns, and potential improvements.

## 1. Project Overview

**Name:** Skool-Courses-scraper  
**Language:** Go (version 1.24.2)  
**Purpose:** Extract courses, modules, and videos from Skool classrooms for offline access  
**Repository Structure:**
- `skool-video-dl.go` - Main source file
- `go.mod` & `go.sum` - Go module files
- `README.md` - Documentation
- `downloads/` - Output directory

## 2. Technical Specifications

### 2.1 Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| Go | 1.24.2 | Programming language |
| chromedp | v0.13.6 | Browser automation |
| yt-dlp | External | Video downloading |
| Google Chrome | External | Headless browsing |

### 2.2 Key Data Structures

| Structure | Purpose | Key Fields |
|-----------|---------|------------|
| Config | Runtime configuration | SkoolURL, Email, Password, OutputDir, Wait, Headless, Debug |
| Course | Basic course information | Title, URL |
| CourseData | Extended course data | Title, URL, Modules[] |
| ModuleInfo | Basic module information | ID, Title, URL |
| ModuleData | Extended module data | Title, URL, Description, Videos[] |
| VideoRecord | Video download record | URL, Filename |
| TiptapNode | Rich text content node | Type, Text, Marks[], Attrs, Content[] |
| Mark | Text formatting | Type, Attrs |

### 2.3 Command-Line Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| -url | Yes | - | Skool classroom URL |
| -email | Yes | - | Email for Skool login |
| -password | Yes | - | Password for Skool login |
| -output | No | "downloads" | Download directory |
| -wait | No | 5 | Wait time (seconds) after navigation |
| -headless | No | true | Run Chrome headless |
| -debug | No | false | Show debug logs |

## 3. Functional Analysis

### 3.1 Core Functionality

The application performs the following key functions:

1. **Authentication**
   - Uses chromedp to navigate to Skool login page
   - Submits credentials via form fields
   - Waits for successful login

2. **Course Discovery**
   - Extracts course information from Skool's __NEXT_DATA__ JSON
   - Supports both classroom-wide listing and single course URLs
   - Parses course titles and URLs

3. **Module Enumeration**
   - For each course, extracts module information
   - Captures module IDs, titles, and URLs
   - Builds a structured representation of the course hierarchy

4. **Content Extraction**
   - Parses module descriptions from Tiptap JSON format
   - Extracts embedded video links (Vimeo, Loom)
   - Converts rich text to clean HTML

5. **Video Downloading**
   - Normalizes Vimeo URLs to multiple formats
   - Uses yt-dlp to download videos
   - Implements resumable downloads with file existence checking

6. **HTML Generation**
   - Creates module.html files with formatted content
   - Embeds video players for offline viewing
   - Generates an index.html for navigating all content

### 3.2 Workflow Diagram

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Parse Flags │────▶│  Setup      │────▶│  Login to   │
│ & Config    │     │  Browser    │     │  Skool      │
└─────────────┘     └─────────────┘     └──────┬──────┘
                                               │
┌─────────────┐     ┌─────────────┐     ┌──────▼──────┐
│  Generate   │◀────│  Download   │◀────│  Extract    │
│  HTML Files │     │  Videos     │     │  Content    │
└──────┬──────┘     └─────────────┘     └──────┬──────┘
       │                                       │
       │            ┌─────────────┐            │
       └───────────▶│  Create     │◀───────────┘
                    │  Index      │
                    └─────────────┘
```

## 4. Architectural Analysis

### 4.1 Design Pattern

The application follows a procedural, pipeline-based architecture with sequential processing steps. Key architectural characteristics include:

- **Monolithic Design**: All functionality in a single file
- **Linear Execution Flow**: Sequential processing of courses and modules
- **Stateless Operation**: No persistent state between runs
- **External Tool Integration**: Delegation to yt-dlp for video downloading

### 4.2 Component Diagram

```
┌───────────────────────────────────────────────────────┐
│                  skool-video-dl.go                    │
├───────────────┬───────────────┬───────────────────────┤
│ Configuration │ Browser       │ Course/Module         │
│ Management    │ Automation    │ Discovery             │
├───────────────┼───────────────┼───────────────────────┤
│ Tiptap        │ Video Link    │ HTML                  │
│ Processing    │ Extraction    │ Generation            │
└───────────┬───┴───────────────┴───────────────────────┘
            │
            ▼
┌───────────────────┐          ┌───────────────────┐
│     chromedp      │          │      yt-dlp       │
│  (Browser Engine) │          │  (Video Downloader)│
└───────────────────┘          └───────────────────┘
```

### 4.3 Data Flow

1. **Input**: Command-line arguments → Config structure
2. **Authentication**: Config → Browser session → Authenticated state
3. **Discovery**: Authenticated session → JSON parsing → Course/Module structures
4. **Extraction**: Module data → Tiptap parsing → HTML content + Video links
5. **Download**: Video links → yt-dlp → Local video files
6. **Output**: Content + Videos → HTML generation → File system

## 5. Implementation Details

### 5.1 Key Algorithms

#### Tiptap to HTML Conversion
The application implements a recursive parser to convert Skool's Tiptap JSON format to HTML:
- Handles various node types (heading, paragraph, list, text)
- Processes text formatting (bold, italic, links)
- Maintains hierarchical structure of content

#### Vimeo URL Normalization
A comprehensive algorithm for handling different Vimeo URL formats:
- Extracts video IDs and hashes from various patterns
- Generates multiple URL variants for compatibility
- Implements fallback strategies for download attempts

#### Recursive Content Traversal
Implements depth-first traversal of nested JSON structures:
- Extracts video links from any depth in the content
- Processes nested Tiptap nodes for HTML conversion
- Maintains formatting and structure during conversion

### 5.2 Error Handling

The application employs several error handling strategies:

- **Fail-fast**: Critical errors terminate execution
- **Resumability**: Skips previously downloaded content
- **Logging**: Debug mode for troubleshooting
- **Graceful Degradation**: Continues processing other modules if one fails

### 5.3 File Structure

The application generates the following output structure:

```
downloads/
└── Course Title/
    ├── 01 - Module Title/
    │   ├── video-01.mp4
    │   └── module.html
    ├── 02 - Next Module/
    │   ├── video-01.mp4
    │   └── module.html
    └── ...
└── index.html
```

## 6. Usage Analysis

### 6.1 Intended Use Cases

- Personal archiving of purchased Skool content
- Offline access to educational materials
- Backup of course content for personal reference
- Internal training content archiving (for owned content)

### 6.2 Typical Workflow

1. Install dependencies (Go, yt-dlp, Chrome)
2. Build the application
3. Run with required credentials and URL
4. Access downloaded content in the output directory

### 6.3 Legal and Ethical Considerations

The tool is intended only for content the user has legal rights to access:
- ✅ Personal access to purchased content
- ✅ Internal training content owned by the user
- ❌ Unauthorized access to others' content
- ❌ Distribution of private video content

## 7. Evaluation

### 7.1 Strengths

- **Functionality**: Successfully extracts Skool content
- **Flexibility**: Supports classroom-wide and single-course scraping
- **Content Quality**: Good conversion of rich text to HTML
- **Video Support**: Comprehensive handling of Vimeo URLs
- **Resumability**: Skips previously downloaded content
- **Output Structure**: Clean, organized file hierarchy

### 7.2 Limitations

- **Monolithic Design**: Single file limits maintainability
- **Sequential Processing**: No parallelization for performance
- **Error Recovery**: Limited handling of network issues
- **Progress Tracking**: No feedback during long operations
- **External Dependencies**: Requires external tool installation
- **Configuration**: Command-line only, no config files

### 7.3 Improvement Opportunities

#### Short-term Improvements
1. **Modularization**: Split into multiple files/packages
2. **Enhanced Error Handling**: Add retry mechanisms
3. **Progress Reporting**: Add visual feedback
4. **Configuration Files**: Support for config files
5. **Consistent Language**: Standardize on English or French

#### Long-term Enhancements
1. **Concurrent Processing**: Parallelize downloads
2. **Web Interface**: Add a simple UI
3. **Plugin Architecture**: Support for custom processors
4. **API Integration**: Direct API access
5. **Testing Framework**: Add unit and integration tests
6. **Containerization**: Package as Docker container

## 8. Conclusion

The Skool-Courses-scraper is a functional tool that effectively accomplishes its goal of extracting and archiving Skool classroom content. While its monolithic design limits extensibility, the implementation is straightforward and effective for its intended purpose.

The tool demonstrates good understanding of Skool's platform structure, particularly in parsing the Tiptap format and handling various Vimeo URL patterns. With architectural improvements and enhanced error handling, it could evolve into a more robust solution.

For users with legitimate access to Skool content who need offline access, this tool provides a valuable service when used within the ethical and legal guidelines outlined in the documentation.