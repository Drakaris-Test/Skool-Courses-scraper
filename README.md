# Skool Courses Scraper

A Go script that automatically extracts all modules and videos from a Skool classroom, including Vimeo-hosted content (if you have access rights).

---

## ✅ Features

- Automatically scrapes all courses and modules from a Skool classroom
- Downloads embedded videos via [yt-dlp](https://github.com/yt-dlp/yt-dlp) (Vimeo fully supported)
- Generates clean HTML pages for each module (text + video)
- Supports all Vimeo link formats (`/video/ID`, `/ID/hash`, shared links, etc.)
- Fully terminal-based, fast, and portable
- Resumes gracefully: previously downloaded modules or videos are skipped if their files are present and non-empty

---

## ⚙️ Requirements

### Software:

- [Go](https://golang.org/dl/) (v1.18 or higher recommended)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) (used to download videos)
- [Google Chrome](https://www.google.com/chrome/) (used in headless mode via `chromedp`)

### Install yt-dlp:

```bash
brew install yt-dlp
# or
pip install -U yt-dlp
🚀 How to Use
1. Clone the repository
bash
Copier
Modifier
git clone https://github.com/Sniv3lbe/Skool-Courses-scraper.git
cd Skool-Courses-scraper
2. Build the script
bash
Copier
Modifier
go build -o skool-courses-scraper
3. Run the scraper
bash
Copier
Modifier
./skool-courses-scraper \
  -url "https://www.skool.com/your-classroom/classroom" \
  -email "your.email@example.com" \
  -password "your_password"
The script will:

Log into your Skool account

List all courses and modules

Download associated videos (if available and accessible)

Generate an HTML page per module in the downloads/ folder

🔒 Legal & Ethical Use
⚠️ This tool must only be used for content you legally have the right to export.
Never use it to steal, resell, or redistribute paid or private content without proper permission.

Allowed use cases:

✔️ Personal access to purchased Skool classrooms

✔️ Internal training content you own

❌ Accessing or downloading others’ content without permission

❌ Distributing private video content

🧪 Testing First
Before exporting a full classroom, test on a single module:

bash
Copier
Modifier
./skool-courses-scraper \
  -url "https://www.skool.com/your-classroom/classroom" \
  -email "your.email@example.com" \
  -password "your_password" \
  -debug
📂 Output Structure
vbnet
Copier
Modifier
downloads/
└── Course Title/
    ├── 01 - Module Title/
    │   ├── video-01.mp4
    │   └── module.html
    ├── 02 - Next Module/
    └── ...