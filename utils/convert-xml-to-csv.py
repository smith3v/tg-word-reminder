#!/usr/bin/env python3

# The script converts the XML dictionary from AnkiApp to a CSV file that can be used to import the words into the bot.

import xml.etree.ElementTree as ET
import csv

def extract_text(element):
    """Extract text from an XML element, handling <p> tags."""
    if element is not None:
        # Check if the element has <p> tags
        p_tags = element.findall('p')
        if p_tags:
            # If <p> tags are present, return the text from the first <p> tag
            return p_tags[0].text
        else:
            # Otherwise, return the text directly from the element
            return element.text
    return None

def main():
    # Parse the XML file
    tree = ET.parse('inburgeringonline.xml')
    root = tree.getroot()

    # Open a CSV file for writing
    with open('inburgeringonline.csv', 'w', newline='', encoding='utf-8') as csvfile:
        writer = csv.writer(csvfile, delimiter='\t')

        # Iterate through each card in the XML
        for card in root.findall('.//card'):
            front = card.find("rich-text[@name='Front']")
            back = card.find("rich-text[@name='Back']")
            # Extract text from front and back
            front_text = extract_text(front)
            back_text = extract_text(back)
            # Check if both front and back exist
            if front is not None and back is not None:
                writer.writerow([front_text, back_text])
                print(f"Front: {front.text}, Back: {back.text}")  # Debugging output

    print("CSV file created successfully.")

if __name__ == "__main__":
    main()