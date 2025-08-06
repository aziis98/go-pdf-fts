#!/usr/bin/env python3
"""
PDF text extraction script using PyMuPDF (pymupdf) bindings.
Extracts text from each page of a PDF file and prints it.
"""

import sys
import argparse
import pymupdf  # PyMuPDF


def extract_text_from_pdf(pdf_path, verbose=False):
    """
    Extract text from each page of a PDF file.

    Args:
        pdf_path (str): Path to the PDF file
        verbose (bool): Whether to print verbose output

    Returns:
        list: List of strings, one for each page
    """
    try:
        # Open the PDF document
        doc = pymupdf.open(pdf_path)

        if verbose:
            print(f"Opened PDF: {pdf_path}")
            print(f"Number of pages: {len(doc)}")
            print("-" * 50)

        pages_text = []

        # Extract text from each page
        for page_num in range(len(doc)):
            page = doc.load_page(page_num)
            text = page.get_text()

            if verbose:
                print(f"Page {page_num + 1}:")
                print(f"Characters: {len(text)}")
                print(
                    f"Text preview: {repr(text[:100])}{'...' if len(text) > 100 else ''}")
                print("-" * 30)

            pages_text.append(text)

        doc.close()
        return pages_text

    except Exception as e:
        print(f"Error processing PDF {pdf_path}: {e}", file=sys.stderr)
        return []


def extract_text_with_blocks(pdf_path, verbose=False):
    """
    Extract text using text blocks for better formatting.

    Args:
        pdf_path (str): Path to the PDF file
        verbose (bool): Whether to print verbose output

    Returns:
        list: List of strings, one for each page
    """
    try:
        doc = pymupdf.open(pdf_path)

        if verbose:
            print(f"Opened PDF: {pdf_path}")
            print(f"Number of pages: {len(doc)}")
            print("-" * 50)

        pages_text = []

        for page_num in range(len(doc)):
            page = doc.load_page(page_num)

            # Get text blocks (preserves formatting better)
            blocks = page.get_text("blocks")

            page_text = ""
            for block in blocks:
                if len(block) >= 5 and block[4]:  # Text block
                    page_text += block[4] + "\n"

            if verbose:
                print(f"Page {page_num + 1}:")
                print(f"Text blocks: {len(blocks)}")
                print(f"Characters: {len(page_text)}")
                print(
                    f"Text preview: {repr(page_text[:100])}{'...' if len(page_text) > 100 else ''}")
                print("-" * 30)

            pages_text.append(page_text)

        doc.close()
        return pages_text

    except Exception as e:
        print(f"Error processing PDF {pdf_path}: {e}", file=sys.stderr)
        return []


def main():
    parser = argparse.ArgumentParser(
        description="Extract text from PDF pages using PyMuPDF")
    parser.add_argument("pdf_file", help="Path to the PDF file")
    parser.add_argument("-v", "--verbose",
                        action="store_true", help="Verbose output")
    parser.add_argument("-b", "--blocks", action="store_true",
                        help="Use text blocks for better formatting")
    parser.add_argument("-p", "--page", type=int,
                        help="Extract text from specific page only (1-indexed)")
    parser.add_argument("-o", "--output", help="Output file (default: stdout)")

    args = parser.parse_args()

    # Choose extraction method
    if args.blocks:
        pages_text = extract_text_with_blocks(args.pdf_file, args.verbose)
    else:
        pages_text = extract_text_from_pdf(args.pdf_file, args.verbose)

    if not pages_text:
        print("No text extracted or error occurred", file=sys.stderr)
        sys.exit(1)

    # Determine output
    output_file = None
    if args.output:
        output_file = open(args.output, 'w', encoding='utf-8')
        output = output_file
    else:
        output = sys.stdout

    try:
        if args.page:
            # Extract specific page (convert to 0-indexed)
            page_idx = args.page - 1
            if 0 <= page_idx < len(pages_text):
                print(f"=== Page {args.page} ===", file=output)
                print(pages_text[page_idx], file=output)
            else:
                print(
                    f"Error: Page {args.page} not found. PDF has {len(pages_text)} pages.", file=sys.stderr)
                sys.exit(1)
        else:
            # Extract all pages
            for i, text in enumerate(pages_text):
                print(f"=== Page {i + 1} ===", file=output)
                print(text, file=output)
                if i < len(pages_text) - 1:
                    print("\n" + "="*50 + "\n", file=output)

    finally:
        if output_file:
            output_file.close()


if __name__ == "__main__":
    main()
