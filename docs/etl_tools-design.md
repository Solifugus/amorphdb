# AmorphDB ETL Tools

This specifies the design for a set of ETL tools for AmorphDB.
The emphasis is on business related data, for example: financial, inventory, schedules, etc.

This should include import/export with the following file types:
- Delimited
- JSON
- XML
- Fixed-Width

## Import/Export of Delimited Files

TODO: 
  This should default to comma delimited recognizing quoted data (so ignore commas within quotes).
  However, it should be possible to specify the delimiter and optionally not recognize quoted values as such.

## Import/Export of JSON Files

TODO:
  It should be possible to translate structured results.

## Import/Export of XML Files

TODO:
  It should be possible to translate structured results.

## Import/Export of Fixed-Width Files


**Import**
For fixed-width files, we will use a special notation (TODO:come up with a name for this notation).
The idea is to identify sections and fields and/or subsections within each.
We do so by identifying the start and, optionally, end of each section then each field within
Sections and fields are identified via relational anchors: what surrounds it.

A few general rules:
- The start and end of a file implicitly define the start and end of the root section.
- If a section end is not specified, it ends where the next section at the same or higher level is identified.
- The end of file ends all sections.


Example:
```
section root_section_name:
    field id: left 3 up 1-100: "ID"
    field file_date: right 10 up 1-100: date
    section transactions starts("==== transactions ====", ends("==== end of transactions ==="):
        field tran_type: left 2 up 1-50: "type"
        field tran_amount: right 30 up 1-50: "amount"
        field tax: right flush: money
        
```

In the above example, there is a root section (the file itself, implicitly) and a subsection (named "transactions").
Each field is identified within its section (which might be a subsection).
The fields are anchor relationally to themmselves.
They may be anchored to one of three things:
- text literal
- regular expression (example: /..find_this../ or /..find_this../..transform_to_this../).
- a data type (known formats)
Actually, the anchor may be a comma separated list of one or more of these--any of which may match (OR--not AND).
The number of spaces may be specified as a specific number, a range of numbers, or open-ended on one end (e.g. left 2 vs. left 2-5 vs left 2- vs left -5).
Also, the keyword "flush" means all the way (left, right, up, or down).
If it's flush up or down, it means the start or end of the section.
So technically, we could specify the root section `section myfile starts(up flush) ends (down flush):`

This should read in structured data from simple to complex fixed-width files.
This is important especially in the Financial Services industry with vendors commonly using archaic flat file formats (e.g. PSCU).

**Export**
TODO
