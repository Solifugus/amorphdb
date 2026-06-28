# Learning AmorphDB

## Design Philosophy

This book teaches AmorphDB to lightly skilled and junior technical people working
in business operations — sys admins, analysts, and developers. These readers are
thinking: "How can we make best use of this to solve our problems with minimal
effort?" The book answers that question directly.

**Show, then explain, then expand.** Every new concept begins with a working
example the reader can type into the REPL and see results. Explanation follows
the example. Expansion (theory, edge cases, additional patterns) follows the
explanation. Never lead with theory.

**Practical, not philosophical.** Theory is included where it helps the reader
use AmorphDB effectively. The temporal model, the tree hierarchy, the reactive
watcher system — these are explained in terms of what they let the reader *do*,
not as computer science concepts. Time is more valuable than gold.

**The HTML test.** When HTML came out, 12-year-old kids were writing cool and
useful web pages. If AmorphDB cannot be quick to learn and easy to build things
of practical use, it will not catch on. This book must deliver that experience:
the distance from "I know nothing" to "I made something useful" should be as
short as possible.

**All examples must work.** Every code listing in the book must be valid MBL
that produces the output shown. No pseudocode, no hand-waving, no "imagine
this works." If a feature is not yet implemented, the example is not included.


## Audience

- **Sys admins** who manage infrastructure and want to understand deployment,
  mesh management, and operational concerns.
- **Business analysts** who work with data and want to query, track changes,
  build reports, and automate workflows.
- **Junior developers** who build internal tools and want to write procedures,
  watchers, and integrations.

These readers are comfortable with a command line. They may or may not have
programming experience. They are not computer scientists and do not want to
become one — they want to solve business problems.


## Current State of AmorphDB

AmorphDB is a new product under active development. The book and the product
are intended for simultaneous release. At time of writing:

- The REPL command-line client (`amorph`) is the only client built.
- The PWA browser client does not yet exist.
- The design specification will be revised as remaining features are built.

The book is written around the REPL as the primary interface. Chapters covering
the PWA client, network sub-library, and other unbuilt features are deferred
until those features are complete. The book design will be updated with a
revised specification at that time.


## Publication

The final version is intended for publication through Amazon KDP.

- **Format:** DOCX for interior, generated programmatically
- **Trim size:** 6" × 9" (standard technical book)
- **Margins:** 0.75" gutter (binding side), 0.5" outside, 0.75" top/bottom
- **Page numbers:** Outside edge (left on verso/left-hand pages, right on
  recto/right-hand pages)
- **Body font:** Serif, readable at length (e.g., Source Serif Pro, 10.5pt)
- **Code font:** Monospace, sized to fit the text block (e.g., Source Code Pro,
  9pt)
- **Heading font:** Sans-serif for contrast (e.g., Source Sans Pro)
- **Table of Contents:** Auto-generated, included in front matter
- **ISBN:** Provided at generation time; embedded in title page and metadata


## Book Structure

### Front Matter
- Title Page (title, subtitle, author, edition)
- Copyright Page
- Table of Contents
- Preface (who this book is for, how to use it, conventions)

### Part I: Getting Started
- Chapter 1: What Is AmorphDB?
- Chapter 2: Installation and First Run

### Part II: Tutorial
- Chapter 3: The REPL — Your First Session
- Chapter 4: Records and Lists
- Chapter 5: Time Travel — Temporal Queries
- Chapter 6: Types and Values
- Chapter 7: Procedures
- Chapter 8: Watchers — Reactive Automation
- Chapter 9: Templates and Heritability
- Chapter 10: Stamps, Filters, and Permissions
- Chapter 11: The Computer — Files, Shell, and System Access

### Part III: Cookbook
- Chapter 12: Employee Directory with Change History
- Chapter 13: Inventory Tracker with Reorder Alerts
- Chapter 14: Loan Payment Ledger with Full Audit Trail
- Chapter 15: Customer Onboarding Workflow
- Chapter 16: Daily Report Automation
- Chapter 17: Compliance Audit System
- Chapter 18: Multi-Node Deployment Patterns
- (Additional recipes TBD — see Cookbook Design Notes below)

### Part IV: Administration
- Chapter 19: Standalone and Mesh Deployment
- Chapter 20: Agents and Identity
- Chapter 21: Mesh Operations — Joining, Bridging, Detaching
- Chapter 22: Maintenance — Compaction, Purge, Monitoring
- Chapter 23: Security and Encryption
- Chapter 24: Configuration Reference (amorphd.toml)

### Part V: Reference
- MBL Quick Reference (keywords, operators, precedence)
- Data Types
- Meta Attributes
- Built-in Procedures (I/O, text, math, time, type, collection)
- Collection Operations
- Control Flow
- amorphd Reference
- amorphctl Command Reference
- amorph Client Reference
- Type Coercion Rules
- Reserved Words
- Glossary

### Back Matter
- Index


## Chapter Design Notes

### Part I: Getting Started

**Chapter 1: What Is AmorphDB?**

Not a theory lecture. Opens with a side-by-side: "Here is how you track a
customer's address in a traditional database. Here is how you track it in
AmorphDB." The reader sees immediately that AmorphDB keeps the full history
automatically — no audit tables, no triggers, no extra work.

Then: a one-page overview of the three big ideas (temporal history, tree
hierarchy, reactive watchers) explained in terms of what problems they solve.
Keep it to 5–8 pages total. The reader should finish this chapter wanting to
install it.

**Chapter 2: Installation and First Run**

Just enough administration to get running. Install, start amorphd in standalone
mode, launch the REPL, write a value, read it back. Confirm it works. Done.
Everything else about administration comes later, after the reader knows the
language and cares about deployment.


### Part II: Tutorial

The tutorial is a guided workshop, not a textbook. Each chapter ends with
something working and useful. "By the end of this chapter, you will have
built X" — and X is always something the reader would actually want.

A single running example threads through the tutorial: a small business
system (e.g., a shop with products, orders, and customers) that grows in
sophistication chapter by chapter. By the end of the tutorial, the reader
has a working multi-feature application built entirely in the REPL.

**Chapter 3: The REPL — Your First Session.** Write values, read them back.
The `my` scope. Dot-path traversal. Bare identifiers as local variables vs.
`my.*` as persistent data. The reader builds a simple contact list.

**Chapter 4: Records and Lists.** Structured data. Record literals with `{}`.
Lists with numeric indexing. `..count`, `..append`, `..prepend`, `..remove`,
`..combine`. The reader builds a product catalog.

**Chapter 5: Time Travel — Temporal Queries.** The core differentiator. Change
a value, see the old value is still there. `@time`, bracket expressions with
`@`, temporal filtering. The reader adds price history to their product
catalog and queries it across time.

**Chapter 6: Types and Values.** Text (quote adjacency), Number, Boolean, Time
literals, Money, Unknown (and why it matters), References with `(link)`.
Truthiness rules. Type coercion. The `?=` safe comparison operators.

**Chapter 7: Procedures.** Definition, calling, parameters, return values.
Local scope vs. persistent scope. Sub-attributes as persistent local storage.
Anonymous procedures. The reader writes utility procedures for their shop
system.

**Chapter 8: Watchers — Reactive Automation.** The second core differentiator.
`watch` definition, `(quietly)`, multi-path watchers, append watchers. The
reader adds automatic low-stock alerts and order processing to their shop.
This is where AmorphDB starts to feel like magic.

**Chapter 9: Templates and Heritability.** `new`, `(copy)`, `(link)`,
`(reset)`, `(exclude)`. Per-field overrides. Global modifiers. The reader
creates product and customer templates for their shop.

**Chapter 10: Stamps, Filters, and Permissions.** Metadata that follows data.
Agent stamps, custom stamp attributes, stamp queries. Filters for controlling
your own view. Permissions for controlling access. The reader adds
department-level access control to their shop system.

**Chapter 11: The Computer — Files, Shell, and System Access.** `my.computer`
as a virtual mount. File I/O (read, write, import, export). Shell execution.
The reader imports product data from a CSV file and exports a report.

> **Deferred chapters (pending feature completion):**
> - Embed and Structural Composition
> - The Network Sub-Library (HTTP, SSE, PWA serving)
> - Building a PWA Client


### Part III: Cookbook

The cookbook is the sales floor of the book. A reader who flips through the
cookbook and sees a recipe that looks like their exact problem — solved in 15–30
lines of MBL — is a converted user. Every recipe must feel like something the
reader could copy, adapt, and deploy in an afternoon.

**Recipe format:**
1. **The Problem** — 2–3 sentences describing a real business situation.
2. **The Solution** — Complete, working MBL code. The reader can paste this
   into a REPL and have it running.
3. **How It Works** — Walk through the code, explaining each piece. Reference
   tutorial chapters for deeper explanation.
4. **Variations** — Quick modifications for related problems.

**Cookbook Design Notes:**

The recipes should be chosen to cover the most common business operations
scenarios and to showcase AmorphDB's unique strengths (temporal history,
reactive automation, decentralized mesh). Each recipe should be self-contained
but may reference other recipes for composition.

Candidate recipes (to be refined through discussion):

- Employee directory with full change history and organizational reporting
- Inventory tracker with automatic reorder alerts (watcher-driven)
- Loan or payment ledger with complete audit trail
- Customer onboarding workflow with status tracking
- Daily/weekly/monthly automated report generation via world.clock watchers
- Compliance audit system (who changed what, when, with stamp-based queries)
- Multi-node deployment patterns (mesh setup for different team sizes)
- Configuration management with change tracking
- Approval workflow (request → review → approve/reject, with history)
- Time-series data collection and analysis (sensor readings, metrics)
- Access-controlled team workspace with permission tiers
- Data import/export pipeline (CSV/JSON → AmorphDB → reports)

> **Deferred recipes (pending feature completion):**
> - External API integration (inbound webhooks, outbound HTTP)
> - Cross-mesh data bridging and synchronization
> - PWA client recipes
> - Fixed-width file import with ARI


### Part IV: Administration

Administration comes after the tutorial because the reader doesn't care about
mesh topology until they know the language and want to deploy it. The exception
(basic installation) is handled in Chapter 2.

These chapters are for the sys admin persona. They cover deployment, mesh
lifecycle, security, and operational maintenance. Tone is practical and
procedural: "here is what you do, here is why, here is what to watch for."


### Part V: Reference

The reference section is organized for lookup, not reading. Terse entries,
alphabetical within categories, with cross-references back to the tutorial
chapter that explains each concept in context. A reader who knows what they're
looking for should find it in under 30 seconds.


## Running Example: "The Shop"

A single running example threads through the tutorial chapters. It starts as a
few values in the REPL and grows into a multi-feature system:

- Ch 3: A contact list (names and phone numbers)
- Ch 4: A product catalog (records with name, price, stock)
- Ch 5: Price history queries ("what was the price last month?")
- Ch 6: Rich types (money for prices, time for dates, unknown for missing data)
- Ch 7: Utility procedures (calculate_total, format_receipt)
- Ch 8: Watchers (low-stock alerts, automatic order confirmation)
- Ch 9: Templates (product template, customer template)
- Ch 10: Permissions (manager vs. clerk access levels)
- Ch 11: File I/O (import products from CSV, export daily report)

By the end of the tutorial, the reader has built something real and useful
entirely from the REPL. The cookbook then shows how to apply these same
techniques to other business domains.


## Examples Library

All code examples from the book are collected in a companion examples
directory, organized by chapter. Each example is a standalone `.mbl` file
that can be executed via `amorph script.mbl`. The examples directory is
available for download separately from the book.

Examples are numbered and named: `ch03_01_first_contact.mbl`,
`ch08_03_append_watcher.mbl`, etc.


## Open Questions

- Should the cookbook recipes be fully independent or build on "The Shop"
  scenario? (Current thinking: independent, for maximum reusability.)
- How many cookbook recipes is the right number? (Enough to cover common
  business scenarios without padding. Quality over quantity.)
- Should there be a "Quick Start" card or cheat sheet as a pull-out or
  appendix? (Likely yes — a 2-page MBL quick reference.)
- Cover design, subtitle, and back-cover copy — TBD.
- Exact chapter count will shift as features are completed and the spec
  is revised.
