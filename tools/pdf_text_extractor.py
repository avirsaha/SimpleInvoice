import sys
import pdfplumber

def extract_text_simple(page):
    return page.extract_text() or ""

def extract_text_columns(page):
    words = page.extract_words()
    mid_x = page.width / 2

    left_col, right_col = [], []
    for word in words:
        if word['x0'] < mid_x:
            left_col.append(word)
        else:
            right_col.append(word)

    def group_by_line(words):
        lines = {}
        for w in words:
            y = round(w['top'], 1)
            lines.setdefault(y, []).append(w)
        return [sorted(line, key=lambda w: w['x0']) for y, line in sorted(lines.items())]

    def reconstruct(lines):
        return "\n".join(" ".join(w['text'] for w in line) for line in lines)

    left_lines = group_by_line(left_col)
    right_lines = group_by_line(right_col)

    return reconstruct(left_lines) + "\n\n" + reconstruct(right_lines)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python pdf_text_extractor.py <file.pdf> [--mode=simple|columns]")
        sys.exit(1)

    pdf_path = sys.argv[1]
    mode = "simple"
    if len(sys.argv) > 2 and sys.argv[2].startswith("--mode="):
        mode = sys.argv[2].split("=")[1]

    with pdfplumber.open(pdf_path) as pdf:
        if len(pdf.pages) == 0:
            print("")
            sys.exit(0)

        page = pdf.pages[-1]

        if mode == "columns":
            print(extract_text_columns(page))
        else:
            print(extract_text_simple(page))

