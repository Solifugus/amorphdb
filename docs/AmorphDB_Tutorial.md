# AmorphDB Tutorial: Complete Guide to Temporal Database Programming

*A comprehensive guide to building temporal applications with AmorphDB*

## Table of Contents

1. [Introduction and Setup](#1-introduction-and-setup)
2. [MBL Language Basics](#2-mbl-language-basics)
3. [Working with Temporal Data](#3-working-with-temporal-data)
4. [Data Organization and Structure](#4-data-organization-and-structure)
5. [Reactive Programming with Watchers](#5-reactive-programming-with-watchers)
6. [Object-Oriented Programming](#6-object-oriented-programming)
7. [Distributed Systems](#7-distributed-systems)
8. [Security and Permissions](#8-security-and-permissions)
9. [Advanced Features](#9-advanced-features)
10. [Real-World Applications](#10-real-world-applications)
11. [Performance and Optimization](#11-performance-and-optimization)
12. [Troubleshooting](#12-troubleshooting)

---

## 1. Introduction and Setup

### What is AmorphDB?

AmorphDB is a temporal tree-graph database that preserves the complete history of your data. Unlike traditional databases that overwrite information, AmorphDB records every change with full provenance, enabling you to:

- Query data as it existed at any point in time
- Track the complete evolution of information
- Build reactive applications that respond to data changes
- Maintain comprehensive audit trails automatically

### Core Concepts

Before we begin, let's understand AmorphDB's fundamental concepts:

- **Temporal-First**: Every value has a complete history
- **Hierarchical**: Data is organized as a tree of attributes
- **Reactive**: Code can automatically respond to data changes
- **Distributed**: Data scales across multiple nodes seamlessly

### Installation

First, let's get AmorphDB running on your system.

#### Prerequisites
- Go 1.21 or later
- At least 8GB RAM
- 100GB+ available storage

#### Build from Source

```bash
# Clone the repository
git clone https://github.com/yourusername/amorphdb.git
cd amorphdb

# Build all components
make build

# Or build manually
go build ./cmd/amorphd    # Database daemon
go build ./cmd/amorph     # Interactive client
go build ./cmd/amorphctl  # Administrative tool
```

#### Start Your First Database

```bash
# Start the database daemon
./bin/amorphd

# In another terminal, start the interactive client
./bin/amorph
```

You should see the AmorphDB prompt:

```
Connected to amorphd on local socket
Agent: kalevo

amorph>
```

### Your First Command

Let's store and retrieve some data:

```mbl
amorph> my.first_value = "Hello AmorphDB!"
amorph> my.first_value
"Hello AmorphDB!"
```

Congratulations! You've just stored your first temporal value.

---

## 2. MBL Language Basics

Modern Business Language (MBL) is AmorphDB's domain-specific language designed for temporal and hierarchical data operations.

### Basic Syntax

#### Variables and Assignment

```mbl
# Simple assignment
name = "Alice"
age = 25
active = true

# Hierarchical assignment
user.name = "Bob"
user.age = 30
user.profile.email = "bob@example.com"
```

#### Data Types

MBL supports several built-in types:

```mbl
# Text (UTF-8 strings)
greeting = "Hello World"
multiline = """This is a
multi-line string
with quotes "inside" """

# Numbers (always high-precision floating point)
price = 19.99
quantity = 100
total = price * quantity

# Time (UTC timestamps)
now = @2026-03-10 14:30:00
date_only = @2026-03-10
time_only = @14:30:00

# Money (with automatic currency conversion)
cost = ¤USD19.99
euros = ¤EUR15.50

# Boolean
ready = true
complete = false
```

#### Scope Resolution

MBL uses prefixes to determine where data is stored:

```mbl
# Personal data (stored in your home directory)
my.settings.theme = "dark"

# Relative to current scope
.local_variable = "temporary"

# Explicit path traversal
my.world.market.stocks.AAPL.price = 150.25
```

#### Comments

```mbl
# Single line comment
x = 5 # inline comment

## Multi-line comment
This spans multiple
lines until ##

### Nested comments can contain ## symbols ###
```

### Operators

#### Arithmetic

```mbl
result = 10 + 5    # Addition: 15
result = 10 - 3    # Subtraction: 7
result = 4 * 3     # Multiplication: 12
result = 15 / 3    # Division: 5
result = 17 % 5    # Modulo: 2
```

#### String Operations

```mbl
# Concatenation
full_name = "John" & " " & "Doe"

# Text operations
length = "Hello"..length                    # 5
upper = "hello"..upper                      # "HELLO"
words = "apple,banana,orange"..split(",")   # ["apple", "banana", "orange"]
```

#### Comparison

```mbl
# Standard comparisons
x = 5
result = x > 3     # true
result = x ?= 5    # true (equality)
result = x != 4    # true (not equal)

# Time comparisons
today = @2026-03-10
yesterday = @2026-03-09
result = today > yesterday    # true
```

### Control Flow

#### Conditional Statements

```mbl
temperature = 75

if temperature > 80:
    comfort_level = "hot"
elif temperature > 60:
    comfort_level = "comfortable"
else:
    comfort_level = "cold"
```

#### Loops

```mbl
# For loop with range
for i in 1..5:
    my.numbers[i] = i * i

# For loop with collection
colors = ["red", "green", "blue"]
for color in colors:
    output("Color: " & color)

# While loop
counter = 0
while counter < 10:
    my.data[counter] = "item " & counter
    counter = counter + 1
```

### Practice Exercise 1: Basic Data Entry

Create a simple user profile system:

```mbl
# Create your profile
my.profile.name = "Your Name"
my.profile.email = "you@example.com"
my.profile.age = 25
my.profile.joined = @2026-03-10

# Add some preferences
my.profile.settings.theme = "dark"
my.profile.settings.notifications = true
my.profile.settings.language = "en"

# Verify the data
output(my.profile.name)
output("Joined: " & my.profile.joined)
```

---

## 3. Working with Temporal Data

The true power of AmorphDB lies in its temporal capabilities. Every assignment creates a new instance while preserving the complete history.

### Understanding Temporal Assignment

```mbl
# First, set an initial value
my.account.balance = 1000.00

# Later, update it (this doesn't overwrite, it adds to history)
my.account.balance = 1250.50

# Current value
current = my.account.balance
output("Current balance: " & current)    # 1250.50
```

### Temporal Queries

#### Basic Temporal Access

```mbl
# Access current value
current_price = my.stock.AAPL.price

# Access value as of a specific time
historical_price = my.stock.AAPL.price[@2026-01-01]

# Access value before a certain time
price_before = my.stock.AAPL.price[<@2026-02-01]

# Access value after a certain time
price_after = my.stock.AAPL.price[>@2026-02-01]
```

#### Time Range Queries

```mbl
# All values in a time range
q1_prices = my.stock.AAPL.price[>@2026-01-01, <@2026-04-01]

# Values from the last 30 days
recent_prices = my.stock.AAPL.price[>@2026-02-10]
```

#### Instance Metadata

Every piece of data includes metadata about when it was written and by whom:

```mbl
# Check when data was last updated
last_update = my.account.balance.@timestamp
output("Last updated: " & last_update)

# Check who wrote the data
author = my.account.balance.@author
output("Updated by: " & author)

# Navigate through history
previous_value = my.account.balance.@previous
next_value = my.account.balance.@next
```

### Building a Transaction Log

Let's create a simple banking system that tracks all transactions:

```mbl
# Initialize account
my.banking.account.balance = 1000.00
my.banking.account.opened = @2026-01-01

# Function to make a transaction
procedure make_transaction(amount, description):
    old_balance = my.banking.account.balance
    new_balance = old_balance + amount

    # Update balance (automatically timestamped)
    my.banking.account.balance = new_balance

    # Record transaction details
    my.banking.transactions..append({
        amount: amount,
        description: description,
        old_balance: old_balance,
        new_balance: new_balance,
        timestamp: @now
    })

    output("Transaction: " & description & " (" & amount & ")")
    output("New balance: " & new_balance)

# Make some transactions
make_transaction(250.00, "Salary deposit")
make_transaction(-50.00, "Grocery shopping")
make_transaction(-20.00, "Coffee shop")
make_transaction(1000.00, "Bonus payment")

# View transaction history
output("=== Transaction History ===")
for transaction in my.banking.transactions:
    output(transaction.timestamp & ": " & transaction.description & " (" & transaction.amount & ")")

# Check balance at different times
output("Balance on Jan 15: " & my.banking.account.balance[@2026-01-15])
output("Current balance: " & my.banking.account.balance)
```

### Practice Exercise 2: Project Tracking

Create a project management system that tracks status changes over time:

```mbl
# Initialize a project
my.projects.website.name = "Company Website Redesign"
my.projects.website.status = "planning"
my.projects.website.created = @2026-01-01

# Update status over time
my.projects.website.status = "in_progress"
my.projects.website.assigned_to = "design_team"

my.projects.website.status = "review"
my.projects.website.review_notes = "Looks good, minor adjustments needed"

my.projects.website.status = "completed"
my.projects.website.completed_date = @2026-03-10

# Create a function to show project timeline
procedure show_project_timeline(project_path):
    output("=== Project Timeline ===")

    # Walk through the status history
    current = project_path.status
    while current:
        output(current.@timestamp & ": " & current)
        current = current.@previous
```

---

## 4. Data Organization and Structure

AmorphDB organizes data hierarchically, making it easy to structure complex information.

### Hierarchical Organization

```mbl
# Company structure
my.company.name = "TechCorp"
my.company.founded = @2020-01-01

# Departments
my.company.departments.engineering.head = "Alice Johnson"
my.company.departments.engineering.budget = 500000
my.company.departments.engineering.location = "Building A"

my.company.departments.marketing.head = "Bob Smith"
my.company.departments.marketing.budget = 300000
my.company.departments.marketing.location = "Building B"

# Employees
my.company.employees.001.name = "Alice Johnson"
my.company.employees.001.department = "engineering"
my.company.employees.001.salary = 120000
my.company.employees.001.hired = @2021-03-15

my.company.employees.002.name = "Carol Davis"
my.company.employees.002.department = "engineering"
my.company.employees.002.salary = 95000
my.company.employees.002.hired = @2022-07-01
```

### Dynamic Queries

Use bracket notation for dynamic queries:

```mbl
# Find all engineering employees
engineering_staff = my.company.employees[department = "engineering"]

# Find high earners
high_earners = my.company.employees[salary > 100000]

# Find recent hires
recent_hires = my.company.employees[hired > @2022-01-01]

# Complex queries
senior_engineers = my.company.employees[
    department = "engineering",
    salary > 100000,
    hired < @2022-01-01
]
```

### Working with Collections

#### Numerical Indexing (Lists)

```mbl
# Create a shopping list
my.shopping.list..append("milk")
my.shopping.list..append("bread")
my.shopping.list..append("eggs")

# Access by index
first_item = my.shopping.list[0]    # "milk"
second_item = my.shopping.list[1]   # "bread"

# Range access
first_two = my.shopping.list[0:2]   # ["milk", "bread"]

# List operations
count = my.shopping.list..length    # 3
reversed = my.shopping.list..reverse
sorted = my.shopping.list..sort
```

#### Textual Indexing (Maps)

```mbl
# Configuration settings
my.app.config.database_host = "localhost"
my.app.config.database_port = 5432
my.app.config.debug_mode = true

# Access by key
host = my.app.config["database_host"]
host = my.app.config.database_host    # Same thing

# Dynamic key access
setting_name = "debug_mode"
value = my.app.config[setting_name]
```

### Practice Exercise 3: Inventory Management

Build an inventory system for a small store:

```mbl
# Initialize store
my.store.name = "Tech Gadgets Plus"
my.store.opened = @2025-01-15

# Add inventory items
my.store.inventory.laptops.count = 25
my.store.inventory.laptops.price = 899.99
my.store.inventory.laptops.supplier = "TechSupply Co"
my.store.inventory.laptops.last_restock = @2026-03-01

my.store.inventory.phones.count = 50
my.store.inventory.phones.price = 599.99
my.store.inventory.phones.supplier = "MobileDistrib"
my.store.inventory.phones.last_restock = @2026-03-05

my.store.inventory.tablets.count = 15
my.store.inventory.tablets.price = 399.99
my.store.inventory.tablets.supplier = "TabletSource"
my.store.inventory.tablets.last_restock = @2026-02-28

# Create a sale function
procedure make_sale(product, quantity):
    if my.store.inventory[product].count >= quantity:
        my.store.inventory[product].count = my.store.inventory[product].count - quantity

        # Record the sale
        my.store.sales..append({
            product: product,
            quantity: quantity,
            unit_price: my.store.inventory[product].price,
            total: my.store.inventory[product].price * quantity,
            timestamp: @now
        })

        output("Sold " & quantity & " " & product & " for $" & (my.store.inventory[product].price * quantity))
    else:
        output("Insufficient inventory for " & product)

# Make some sales
make_sale("laptops", 3)
make_sale("phones", 7)
make_sale("tablets", 2)

# Check inventory levels
procedure check_inventory():
    output("=== Current Inventory ===")
    for product in my.store.inventory:
        output(product & ": " & my.store.inventory[product].count & " units")

check_inventory()

# Low stock alert
procedure check_low_stock(threshold):
    output("=== Low Stock Alert (< " & threshold & ") ===")
    for product in my.store.inventory:
        if my.store.inventory[product].count < threshold:
            output("LOW: " & product & " (" & my.store.inventory[product].count & " remaining)")

check_low_stock(20)
```

---

## 5. Reactive Programming with Watchers

Watchers are AmorphDB's reactive programming mechanism. They automatically execute when specified data changes.

### Basic Watchers

```mbl
# Simple watcher that responds to price changes
watch price_alert(my.stocks.AAPL.price):
    if my.stocks.AAPL.price > 200:
        my.alerts..append("AAPL price exceeded $200: " & my.stocks.AAPL.price)
        output("ALERT: AAPL is now $" & my.stocks.AAPL.price)

# Test the watcher
my.stocks.AAPL.price = 180
my.stocks.AAPL.price = 205  # This will trigger the alert
```

### Multi-Attribute Watchers

```mbl
# Watcher that monitors multiple values
watch portfolio_monitor(my.portfolio.total_value, my.portfolio.risk_level):
    total = my.portfolio.total_value
    risk = my.portfolio.risk_level

    if total > 100000 and risk > 0.7:
        my.alerts..append("High risk portfolio with significant value")
        output("WARNING: High-value, high-risk portfolio detected")
    elif total < 10000:
        output("NOTICE: Portfolio value is below $10,000")
```

### Wildcard Watchers

```mbl
# Watch all user activity
watch user_activity_monitor(my.users.*.last_login):
    # This triggers whenever any user's last_login changes
    for user_id in my.users:
        login_time = my.users[user_id].last_login
        if login_time > (@now - 300):  # Last 5 minutes
            output("Recent login: " & my.users[user_id].name & " at " & login_time)
```

### Building a Real-Time Dashboard

Let's create a system monitoring dashboard:

```mbl
# Initialize system metrics
my.system.cpu_usage = 45.2
my.system.memory_usage = 62.8
my.system.disk_usage = 78.1
my.system.network_traffic = 150.5

# CPU monitoring watcher
watch cpu_monitor(my.system.cpu_usage):
    cpu = my.system.cpu_usage

    if cpu > 90:
        my.alerts.critical..append({
            type: "CPU",
            level: "CRITICAL",
            value: cpu,
            timestamp: @now,
            message: "CPU usage critically high: " & cpu & "%"
        })
    elif cpu > 75:
        my.alerts.warning..append({
            type: "CPU",
            level: "WARNING",
            value: cpu,
            timestamp: @now,
            message: "CPU usage high: " & cpu & "%"
        })

# Memory monitoring watcher
watch memory_monitor(my.system.memory_usage):
    memory = my.system.memory_usage

    if memory > 90:
        # Automatic cleanup attempt
        my.system.cleanup_requested = @now
        my.alerts.critical..append({
            type: "MEMORY",
            level: "CRITICAL",
            value: memory,
            timestamp: @now,
            message: "Memory usage critical: " & memory & "% - cleanup initiated"
        })

# Disk monitoring with automated response
watch disk_monitor(my.system.disk_usage):
    disk = my.system.disk_usage

    if disk > 85:
        # Trigger log rotation
        my.system.log_rotation.requested = @now
        my.alerts.warning..append({
            type: "DISK",
            level: "WARNING",
            value: disk,
            timestamp: @now,
            message: "Disk usage high: " & disk & "% - log rotation triggered"
        })

# Simulate system changes
my.system.cpu_usage = 82  # Triggers warning
my.system.memory_usage = 93  # Triggers critical alert and cleanup
my.system.disk_usage = 88  # Triggers log rotation

# View alerts
procedure show_recent_alerts(minutes):
    cutoff = @now - (minutes * 60)
    output("=== Alerts from last " & minutes & " minutes ===")

    for alert in my.alerts.critical[timestamp > cutoff]:
        output("CRITICAL: " & alert.message)

    for alert in my.alerts.warning[timestamp > cutoff]:
        output("WARNING: " & alert.message)

show_recent_alerts(10)
```

### Practice Exercise 4: E-commerce Order Processing

Create an order processing system with automatic workflows:

```mbl
# Order structure
procedure create_order(customer, items):
    order_id = uuid()

    my.orders[order_id].customer = customer
    my.orders[order_id].items = items
    my.orders[order_id].status = "pending"
    my.orders[order_id].created = @now
    my.orders[order_id].total = 0

    # Calculate total
    for item in items:
        my.orders[order_id].total = my.orders[order_id].total + item.price

    output("Created order " & order_id & " for " & customer)
    return order_id

# Automatic order processing workflow
watch order_processor(my.orders.*.status):
    for order_id in my.orders:
        order = my.orders[order_id]

        if order.status ?= "pending":
            # Auto-approve small orders
            if order.total < 100:
                my.orders[order_id].status = "approved"
                my.orders[order_id].auto_approved = true
            else:
                my.orders[order_id].status = "review"

        elif order.status ?= "approved":
            # Move to fulfillment
            my.orders[order_id].status = "fulfillment"
            my.orders[order_id].fulfillment_started = @now

        elif order.status ?= "fulfillment":
            # Simulate fulfillment completion after some time
            if order.fulfillment_started and (@now - order.fulfillment_started) > 300:
                my.orders[order_id].status = "shipped"
                my.orders[order_id].shipped = @now

# Email notification watcher
watch order_notifications(my.orders.*.status):
    for order_id in my.orders:
        order = my.orders[order_id]

        if order.status ?= "shipped" and not order.shipping_email_sent:
            my.emails..append({
                to: order.customer,
                subject: "Order Shipped: " & order_id,
                body: "Your order has been shipped and is on its way!",
                sent: @now
            })
            my.orders[order_id].shipping_email_sent = true

# Test the system
order1 = create_order("alice@example.com", [
    {name: "Book", price: 15.99},
    {name: "Pen", price: 2.50}
])

order2 = create_order("bob@example.com", [
    {name: "Laptop", price: 899.99}
])
```

---

## 6. Object-Oriented Programming

AmorphDB supports object-oriented programming through templates and instantiation with sophisticated inheritance controls.

### Templates and Instantiation

```mbl
# Define a user template
user_template:
    name: "Anonymous"
    email: ""
    age: 0
    created: @now
    active: true

# Create users from the template
alice = new user_template {
    name: "Alice Johnson",
    email: "alice@example.com",
    age: 28
}

bob = new user_template {
    name: "Bob Smith",
    email: "bob@example.com",
    age: 34
}

# Templates can be nested
employee_template:
    user_info: new user_template
    employee_id: ""
    department: "unassigned"
    salary: 0
    hire_date: @now

# Create employees
engineer = new employee_template {
    user_info: {
        name: "Carol Davis",
        email: "carol@techcorp.com",
        age: 26
    },
    employee_id: "EMP001",
    department: "engineering",
    salary: 95000
}
```

### Inheritance Modifiers

AmorphDB provides powerful inheritance control through modifiers:

```mbl
# Template with inheritance modifiers
account_template:
    id: (reset random(1000, 9999)) 0     # Generate new ID each time
    balance: 0.00
    created: @now
    secret_key: (exclude) "default"       # Never inherited
    status: (link) "active"               # Shared across instances
    owner: "unassigned"

# Create accounts
account1 = new account_template {
    owner: "Alice"
    # id will be randomly generated
    # secret_key will be excluded
    # status will be linked to the template
}

account2 = new account_template {
    owner: "Bob"
    # Gets a different random ID
    # Shares status with account1
}

# Changing the template status affects all linked instances
account_template.status = "maintenance"  # Both accounts now show "maintenance"
```

### Complex Inheritance

```mbl
# Base vehicle template
vehicle_template:
    make: "Unknown"
    model: "Unknown"
    year: 2026
    vin: (reset generate_vin()) ""

procedure generate_vin():
    return "VIN" & random(100000, 999999)

# Car template inheriting from vehicle
car_template:
    base: new vehicle_template
    doors: 4
    fuel_type: "gasoline"

# Electric car template
electric_car_template:
    base: new car_template
    fuel_type: "electric"    # Override the base
    battery_capacity: 75     # Add new attribute
    range: 300

# Create specific cars
my_tesla = new electric_car_template {
    base: {
        make: "Tesla",
        model: "Model 3",
        year: 2026
    },
    battery_capacity: 82,
    range: 358
}

my_honda = new car_template {
    base: {
        make: "Honda",
        model: "Civic",
        year: 2026
    },
    doors: 2
}
```

### Practice Exercise 5: Game Character System

Create a role-playing game character system with inheritance:

```mbl
# Base character template
character_template:
    name: "Unnamed"
    level: 1
    health: (reset 100) 100
    experience: 0
    created: @now

    # Base stats
    stats:
        strength: 10
        agility: 10
        intelligence: 10
        vitality: 10

# Class templates
warrior_template:
    base: new character_template
    class: "Warrior"
    equipment:
        weapon: "Iron Sword"
        armor: "Leather Armor"
    stats:
        strength: 15    # Override base
        vitality: 13    # Override base

mage_template:
    base: new character_template
    class: "Mage"
    equipment:
        weapon: "Oak Staff"
        armor: "Robes"
    stats:
        intelligence: 15    # Override base
        agility: 12         # Override base
    spells: ["Magic Missile", "Heal"]

rogue_template:
    base: new character_template
    class: "Rogue"
    equipment:
        weapon: "Steel Dagger"
        armor: "Light Leather"
    stats:
        agility: 15         # Override base
        strength: 12        # Override base
    skills: ["Stealth", "Lockpicking"]

# Create player characters
player1 = new warrior_template {
    base: {
        name: "Thorin Ironshield"
    }
}

player2 = new mage_template {
    base: {
        name: "Elara Starweaver"
    },
    spells: ["Magic Missile", "Heal", "Fireball"]  # Add extra spell
}

# Level up function
procedure level_up(character, stat_increases):
    character.level = character.level + 1
    character.health = character.health + 10

    for stat in stat_increases:
        character.stats[stat] = character.stats[stat] + stat_increases[stat]

    output(character.name & " reached level " & character.level & "!")

# Combat system
procedure attack(attacker, defender):
    damage = attacker.stats.strength + random(1, 10)
    defender.health = defender.health - damage

    my.combat_log..append({
        timestamp: @now,
        attacker: attacker.name,
        defender: defender.name,
        damage: damage,
        defender_health: defender.health
    })

    output(attacker.name & " attacks " & defender.name & " for " & damage & " damage!")

    if defender.health <= 0:
        output(defender.name & " has been defeated!")
        return true

    return false

# Test the system
level_up(player1, {strength: 2, vitality: 1})
```

---

## 7. Distributed Systems

AmorphDB is designed for distributed operation. Let's learn how to work with multiple nodes and distributed data.

### Understanding the Mesh

AmorphDB operates as a peer-to-peer mesh where:
- Each node stores portions of the global data tree
- Zones define which node is responsible for which data
- Consistent hashing automatically distributes load
- Replication provides fault tolerance

### Setting Up a Multi-Node System

#### Node 1 (Primary)

```bash
# Start first node
./amorphd --port=5000 --data-dir=/data/node1
```

#### Node 2 (Join mesh)

```bash
# Start second node and join the mesh
./amorphd --port=5001 --data-dir=/data/node2 --join=localhost:5000
```

#### Node 3 (Join mesh)

```bash
# Start third node
./amorphd --port=5002 --data-dir=/data/node3 --join=localhost:5000
```

### Working with Distributed Data

```mbl
# Connect to the mesh
amorph --node=localhost:5000

# Data is automatically distributed based on path
my.data.west_coast.customers = "Stored on node handling west coast data"
my.data.east_coast.customers = "May be stored on a different node"

# The mesh handles routing automatically
west_customers = my.data.west_coast.customers
east_customers = my.data.east_coast.customers
```

### Zone Management

```mbl
# Check which zones exist
zones = world.cluster.zones

# View zone assignments
for zone in zones:
    output("Zone: " & zone.path & " -> Node: " & zone.authority)

# Force a zone split (administrative operation)
# This would typically be done via amorphctl
```

### Cross-Zone Operations

```mbl
# Operations that span zones are automatically coordinated
my.global_config.database_settings = {
    primary: "node1.example.com",
    replicas: ["node2.example.com", "node3.example.com"],
    backup_frequency: 3600
}

# This data might be replicated across multiple zones
for node in my.global_config.database_settings.replicas:
    output("Replica: " & node)
```

### Practice Exercise 6: Multi-Region Application

Create a globally distributed application:

```mbl
# Regional data centers
my.infrastructure.regions.us_west.datacenter = "San Francisco"
my.infrastructure.regions.us_west.capacity = 1000
my.infrastructure.regions.us_west.load = 67.5

my.infrastructure.regions.us_east.datacenter = "New York"
my.infrastructure.regions.us_east.capacity = 800
my.infrastructure.regions.us_east.load = 82.1

my.infrastructure.regions.europe.datacenter = "Frankfurt"
my.infrastructure.regions.europe.capacity = 600
my.infrastructure.regions.europe.load = 45.3

# Global load balancer that distributes based on capacity
watch global_load_balancer(my.infrastructure.regions.*.load):
    for region in my.infrastructure.regions:
        load = my.infrastructure.regions[region].load
        capacity = my.infrastructure.regions[region].capacity

        if load > 90:
            # High load - redirect traffic
            my.traffic.routing[region] = "reduced"
            my.alerts..append({
                region: region,
                message: "High load in " & region & " - traffic reduced",
                timestamp: @now
            })
        elif load < 50:
            # Low load - can accept more traffic
            my.traffic.routing[region] = "preferred"

# User session routing
procedure create_user_session(user_id, preferred_region):
    # Choose best region based on load and preference
    best_region = preferred_region

    if my.traffic.routing[preferred_region] ?= "reduced":
        # Find alternative region
        for region in my.infrastructure.regions:
            if my.traffic.routing[region] != "reduced":
                best_region = region
                break

    # Create session
    session_id = uuid()
    my.user_sessions[session_id].user_id = user_id
    my.user_sessions[session_id].region = best_region
    my.user_sessions[session_id].created = @now

    output("Session " & session_id & " created in " & best_region & " for user " & user_id)

    return session_id

# Test the system
create_user_session("user123", "us_west")
create_user_session("user456", "europe")

# Simulate high load
my.infrastructure.regions.us_east.load = 95  # Triggers load balancer
```

---

## 8. Security and Permissions

AmorphDB provides comprehensive security through permissions, filters, and stamps.

### Agent Identity and Authentication

```mbl
# Your identity is automatically established when you connect
my_identity = ~.@identity
output("Connected as: " & my_identity)

# Personal stamp is applied to all your writes
~.stamp.role = "developer"
~.stamp.department = "engineering"
~.stamp.clearance = "internal"
```

### Setting Permissions

```mbl
# Set read permissions on your project data
my.projects.secret_project.@read = (agent.department ?= "engineering")

# Set write permissions (more restrictive)
my.projects.secret_project.@write = (
    agent.department ?= "engineering" and
    agent.role ?= "senior_developer"
)

# Set expansion permissions (who can add new data)
my.projects.secret_project.@expand = (agent.role ?= "team_lead")

# Hierarchical permissions (applied to all sub-data)
my.company.@read = (agent.clearance in ["internal", "confidential", "secret"])
my.company.@write = (agent.role in ["manager", "admin"])
```

### Filters for Data Visibility

```mbl
# Set your personal filter to only see relevant data
~.filter = {
    department: ~.stamp.department,    # Only see your department's data
    clearance: ~.stamp.clearance       # Only see data at your clearance level
}

# Now queries are automatically filtered
engineering_data = my.company.departments.engineering  # Only if you're in engineering
```

### Stamps and Audit Trails

```mbl
# Hierarchical stamps provide context
my.projects.mobile_app.@stamp.project_code = "MOBILE2026"
my.projects.mobile_app.@stamp.budget = 250000

# All writes under this path automatically get stamped
my.projects.mobile_app.features.authentication.status = "complete"
my.projects.mobile_app.features.ui_design.status = "in_progress"

# Check stamp information
auth_stamp = my.projects.mobile_app.features.authentication.@embed
output("Project: " & auth_stamp.project_code)
output("Budget: " & auth_stamp.budget)
output("Author: " & my.projects.mobile_app.features.authentication.@author)
```

### Practice Exercise 7: Secure Document Management

Create a document management system with role-based security:

```mbl
# Document classification system
my.documents.@stamp.classification = "internal"
my.documents.@stamp.department = "hr"

# Set up security levels
my.documents.public.@read = (Anything)
my.documents.public.@write = (agent.role ?= "content_manager")

my.documents.internal.@read = (agent.clearance in ["internal", "confidential", "secret"])
my.documents.internal.@write = (agent.department ?= "hr" or agent.role ?= "manager")

my.documents.confidential.@read = (agent.clearance in ["confidential", "secret"])
my.documents.confidential.@write = (agent.clearance ?= "secret")

# Document creation with automatic stamping
procedure create_document(path, title, content, classification):
    # Set classification stamp
    path.@stamp.classification = classification
    path.@stamp.created_by = ~.@identity
    path.@stamp.created_date = @now

    # Store document content
    path.title = title
    path.content = content
    path.size = content..length
    path.created = @now
    path.last_modified = @now

    output("Document created: " & title & " (" & classification & ")")

# Access control function
procedure check_access(document_path, operation):
    # This would be enforced automatically by the system
    # but we can simulate the check

    user_clearance = ~.stamp.clearance
    user_department = ~.stamp.department
    doc_classification = document_path.@stamp.classification

    can_access = false

    if operation ?= "read":
        if doc_classification ?= "public":
            can_access = true
        elif doc_classification ?= "internal" and user_clearance in ["internal", "confidential", "secret"]:
            can_access = true
        elif doc_classification ?= "confidential" and user_clearance in ["confidential", "secret"]:
            can_access = true

    return can_access

# Create documents
create_document(my.documents.public.employee_handbook,
    "Employee Handbook",
    "Welcome to the company...",
    "public")

create_document(my.documents.internal.hr_policies,
    "HR Policies",
    "Internal procedures...",
    "internal")

create_document(my.documents.confidential.salary_data,
    "Salary Information",
    "Confidential salary data...",
    "confidential")

# Audit function
procedure audit_document_access(days):
    cutoff = @now - (days * 24 * 60 * 60)

    output("=== Document Access Audit (Last " & days & " days) ===")

    for doc_path in my.documents.*.*:
        if doc_path.@timestamp > cutoff:
            output("Accessed: " & doc_path.title)
            output("  By: " & doc_path.@author)
            output("  When: " & doc_path.@timestamp)
            output("  Classification: " & doc_path.@stamp.classification)
```

---

## 9. Advanced Features

### System Operations

MBL provides powerful system operations for data manipulation:

```mbl
# Text operations
message = "Hello World"
length = message..length          # 11
upper = message..upper           # "HELLO WORLD"
words = message..split(" ")      # ["Hello", "World"]

# Number operations
price = 19.99
rounded = price..round           # 20
floor = price..floor            # 19

# Collection operations
numbers = [3, 1, 4, 1, 5]
sorted = numbers..sort          # [1, 1, 3, 4, 5]
unique = numbers..unique        # [3, 1, 4, 5]
length = numbers..length        # 5

# Chaining operations
result = "hello world"..upper..split(" ")..reverse..join("-")
# Result: "WORLD-HELLO"
```

### Advanced Temporal Operations

```mbl
# Temporal aggregation
procedure calculate_daily_average(value_path, days):
    cutoff = @now - (days * 24 * 60 * 60)
    values = value_path[>cutoff]

    total = 0
    count = 0

    for value in values:
        total = total + value
        count = count + 1

    if count > 0:
        return total / count
    else:
        return 0

# Usage
my.metrics.cpu_usage = 45.2
my.metrics.cpu_usage = 67.8
my.metrics.cpu_usage = 52.1
my.metrics.cpu_usage = 78.9

avg_cpu = calculate_daily_average(my.metrics.cpu_usage, 1)
output("Average CPU usage: " & avg_cpu & "%")
```

### Error Handling and Resilience

```mbl
# Procedures that return Unknown values
procedure safe_divide(a, b):
    if b ?= 0:
        return Unknown("division by zero")
    return a / b

# Error handling
result = safe_divide(10, 0)
if result ?= Unknown:
    output("Error: " & result..reason)
else:
    output("Result: " & result)

# Error recovery with watchers
watch error_monitor(my.system.errors.*):
    for error in my.system.errors:
        if error.severity ?= "critical":
            # Automatic recovery attempt
            my.system.recovery.restart_requested = @now
```

### Performance Optimization

```mbl
# Caching expensive operations
procedure expensive_calculation(input):
    # Check cache first
    cache_key = "calc_" & input
    if my.cache[cache_key]:
        return my.cache[cache_key].result

    # Perform calculation
    result = input * input * input + 42

    # Cache the result
    my.cache[cache_key].result = result
    my.cache[cache_key].computed = @now

    return result

# Batch operations for efficiency
procedure batch_update(updates):
    for update in updates:
        my.data[update.key] = update.value

    # Single commit for all updates
    output("Batch updated " & updates..length & " records")

# Use batch operations
updates = [
    {key: "key1", value: "value1"},
    {key: "key2", value: "value2"},
    {key: "key3", value: "value3"}
]

batch_update(updates)
```

### Practice Exercise 8: Analytics Engine

Build a real-time analytics engine:

```mbl
# Event tracking system
procedure track_event(event_type, user_id, properties):
    event_id = uuid()

    my.analytics.events[event_id].type = event_type
    my.analytics.events[event_id].user_id = user_id
    my.analytics.events[event_id].properties = properties
    my.analytics.events[event_id].timestamp = @now

    # Update real-time counters
    my.analytics.counters[event_type].total = my.analytics.counters[event_type].total + 1
    my.analytics.counters[event_type].last_event = @now

# Real-time aggregation watcher
watch analytics_aggregator(my.analytics.events.*):
    # Recalculate metrics when events change
    cutoff_hour = @now - 3600  # Last hour
    cutoff_day = @now - 86400   # Last day

    # Calculate hourly metrics
    for event_type in my.analytics.counters:
        hourly_events = my.analytics.events[type = event_type, timestamp > cutoff_hour]
        daily_events = my.analytics.events[type = event_type, timestamp > cutoff_day]

        my.analytics.metrics[event_type].hourly_count = hourly_events..length
        my.analytics.metrics[event_type].daily_count = daily_events..length
        my.analytics.metrics[event_type].hourly_rate = hourly_events..length / 60  # per minute

# User behavior analysis
procedure analyze_user_behavior(user_id):
    user_events = my.analytics.events[user_id = user_id]

    # Calculate session information
    sessions = []
    current_session = null
    session_timeout = 1800  # 30 minutes

    for event in user_events..sort_by(@timestamp):
        if not current_session or (event.@timestamp - current_session.last_event) > session_timeout:
            # Start new session
            current_session = {
                start: event.@timestamp,
                events: [event],
                last_event: event.@timestamp
            }
            sessions..append(current_session)
        else:
            # Continue current session
            current_session.events..append(event)
            current_session.last_event = event.@timestamp

    return {
        total_events: user_events..length,
        total_sessions: sessions..length,
        avg_session_length: calculate_avg_session_length(sessions)
    }

procedure calculate_avg_session_length(sessions):
    total = 0
    for session in sessions:
        length = session.last_event - session.start
        total = total + length

    if sessions..length > 0:
        return total / sessions..length
    else:
        return 0

# Test the analytics system
track_event("page_view", "user123", {page: "/home"})
track_event("button_click", "user123", {button: "signup"})
track_event("page_view", "user456", {page: "/products"})
track_event("purchase", "user123", {amount: 29.99, product: "premium_plan"})

# Generate reports
procedure generate_daily_report():
    output("=== Daily Analytics Report ===")

    for event_type in my.analytics.metrics:
        metrics = my.analytics.metrics[event_type]
        output(event_type & ": " & metrics.daily_count & " events (" & metrics.hourly_rate & "/min)")

    output("\nTop Users:")
    # User analysis would go here

generate_daily_report()

# Analyze specific user
user_analysis = analyze_user_behavior("user123")
output("User 123 Analysis:")
output("  Total Events: " & user_analysis.total_events)
output("  Sessions: " & user_analysis.total_sessions)
output("  Avg Session: " & (user_analysis.avg_session_length / 60) & " minutes")
```

---

## 10. Real-World Applications

Let's build complete applications that demonstrate AmorphDB's capabilities.

### Application 1: IoT Sensor Network

```mbl
# IoT device registration
procedure register_device(device_id, device_type, location):
    my.iot.devices[device_id].type = device_type
    my.iot.devices[device_id].location = location
    my.iot.devices[device_id].registered = @now
    my.iot.devices[device_id].status = "active"
    my.iot.devices[device_id].last_seen = @now

    output("Registered device: " & device_id & " (" & device_type & ") at " & location)

# Sensor data ingestion
procedure ingest_sensor_data(device_id, sensor_type, value):
    # Store the raw sensor reading
    my.iot.sensor_data[device_id][sensor_type] = {
        value: value,
        timestamp: @now,
        device_status: my.iot.devices[device_id].status
    }

    # Update device last_seen
    my.iot.devices[device_id].last_seen = @now

# Real-time monitoring
watch iot_monitor(my.iot.sensor_data.*.*):
    for device_id in my.iot.devices:
        for sensor_type in my.iot.sensor_data[device_id]:
            latest = my.iot.sensor_data[device_id][sensor_type]

            # Temperature monitoring
            if sensor_type ?= "temperature":
                if latest.value > 75:
                    my.iot.alerts..append({
                        device: device_id,
                        sensor: sensor_type,
                        value: latest.value,
                        alert: "High temperature warning",
                        timestamp: @now
                    })
                elif latest.value < 32:
                    my.iot.alerts..append({
                        device: device_id,
                        sensor: sensor_type,
                        value: latest.value,
                        alert: "Freeze warning",
                        timestamp: @now
                    })

            # Humidity monitoring
            elif sensor_type ?= "humidity":
                if latest.value > 80:
                    my.iot.alerts..append({
                        device: device_id,
                        sensor: sensor_type,
                        value: latest.value,
                        alert: "High humidity detected",
                        timestamp: @now
                    })

# Device health monitoring
watch device_health_monitor(my.iot.devices.*.last_seen):
    offline_threshold = @now - 300  # 5 minutes

    for device_id in my.iot.devices:
        device = my.iot.devices[device_id]

        if device.last_seen < offline_threshold and device.status ?= "active":
            my.iot.devices[device_id].status = "offline"
            my.iot.alerts..append({
                device: device_id,
                alert: "Device went offline",
                last_seen: device.last_seen,
                timestamp: @now
            })

# Data aggregation for analytics
procedure generate_sensor_summary(device_id, hours):
    cutoff = @now - (hours * 3600)

    summary = {
        device: device_id,
        period: hours & " hours",
        sensors: {}
    }

    for sensor_type in my.iot.sensor_data[device_id]:
        recent_readings = my.iot.sensor_data[device_id][sensor_type][timestamp > cutoff]

        if recent_readings..length > 0:
            values = []
            for reading in recent_readings:
                values..append(reading.value)

            summary.sensors[sensor_type] = {
                readings: values..length,
                min: values..min,
                max: values..max,
                avg: values..sum / values..length
            }

    return summary

# Register some devices
register_device("TEMP_001", "temperature_sensor", "Server Room A")
register_device("HUMID_001", "humidity_sensor", "Server Room A")
register_device("TEMP_002", "temperature_sensor", "Warehouse")

# Simulate sensor data
ingest_sensor_data("TEMP_001", "temperature", 72.5)
ingest_sensor_data("HUMID_001", "humidity", 45.2)
ingest_sensor_data("TEMP_002", "temperature", 78.9)  # This will trigger an alert

# Generate summary
summary = generate_sensor_summary("TEMP_001", 24)
output("24-hour summary for TEMP_001:")
for sensor in summary.sensors:
    info = summary.sensors[sensor]
    output("  " & sensor & ": " & info.readings & " readings, avg=" & info.avg)
```

### Application 2: Financial Trading System

```mbl
# Portfolio management
procedure create_portfolio(portfolio_id, initial_balance):
    my.trading.portfolios[portfolio_id].balance = initial_balance
    my.trading.portfolios[portfolio_id].created = @now
    my.trading.portfolios[portfolio_id].total_value = initial_balance
    my.trading.portfolios[portfolio_id].positions = {}

    output("Created portfolio " & portfolio_id & " with $" & initial_balance)

# Market data ingestion
procedure update_market_price(symbol, price):
    my.trading.market_data[symbol].price = price
    my.trading.market_data[symbol].updated = @now

    # Calculate price change
    previous = my.trading.market_data[symbol].price.@previous
    if previous:
        change = price - previous
        change_percent = (change / previous) * 100
        my.trading.market_data[symbol].change = change
        my.trading.market_data[symbol].change_percent = change_percent

# Order execution
procedure place_order(portfolio_id, symbol, quantity, order_type):
    order_id = uuid()
    current_price = my.trading.market_data[symbol].price
    total_cost = current_price * quantity

    portfolio = my.trading.portfolios[portfolio_id]

    if order_type ?= "BUY":
        if portfolio.balance >= total_cost:
            # Execute buy order
            my.trading.orders[order_id].portfolio = portfolio_id
            my.trading.orders[order_id].symbol = symbol
            my.trading.orders[order_id].quantity = quantity
            my.trading.orders[order_id].price = current_price
            my.trading.orders[order_id].type = "BUY"
            my.trading.orders[order_id].status = "FILLED"
            my.trading.orders[order_id].timestamp = @now

            # Update portfolio
            portfolio.balance = portfolio.balance - total_cost
            if portfolio.positions[symbol]:
                portfolio.positions[symbol].quantity = portfolio.positions[symbol].quantity + quantity
            else:
                portfolio.positions[symbol].quantity = quantity
                portfolio.positions[symbol].avg_price = current_price

            output("BUY order filled: " & quantity & " shares of " & symbol & " at $" & current_price)
        else:
            output("Insufficient funds for BUY order")

    elif order_type ?= "SELL":
        current_position = portfolio.positions[symbol].quantity
        if current_position >= quantity:
            # Execute sell order
            my.trading.orders[order_id].portfolio = portfolio_id
            my.trading.orders[order_id].symbol = symbol
            my.trading.orders[order_id].quantity = quantity
            my.trading.orders[order_id].price = current_price
            my.trading.orders[order_id].type = "SELL"
            my.trading.orders[order_id].status = "FILLED"
            my.trading.orders[order_id].timestamp = @now

            # Update portfolio
            portfolio.balance = portfolio.balance + total_cost
            portfolio.positions[symbol].quantity = portfolio.positions[symbol].quantity - quantity

            output("SELL order filled: " & quantity & " shares of " & symbol & " at $" & current_price)
        else:
            output("Insufficient shares for SELL order")

# Risk management watcher
watch risk_monitor(my.trading.portfolios.*.total_value):
    for portfolio_id in my.trading.portfolios:
        portfolio = my.trading.portfolios[portfolio_id]

        # Calculate current total value
        total_value = portfolio.balance
        for symbol in portfolio.positions:
            position = portfolio.positions[symbol]
            current_price = my.trading.market_data[symbol].price
            position_value = position.quantity * current_price
            total_value = total_value + position_value

        # Update total value
        portfolio.total_value = total_value

        # Check for significant losses
        initial_value = portfolio.total_value.@first  # First recorded value
        if initial_value:
            loss_percent = ((initial_value - total_value) / initial_value) * 100

            if loss_percent > 20:  # 20% loss threshold
                my.trading.risk_alerts..append({
                    portfolio: portfolio_id,
                    alert: "Significant loss detected",
                    loss_percent: loss_percent,
                    current_value: total_value,
                    initial_value: initial_value,
                    timestamp: @now
                })

# Portfolio analytics
procedure analyze_portfolio_performance(portfolio_id, days):
    cutoff = @now - (days * 86400)
    portfolio = my.trading.portfolios[portfolio_id]

    # Get value history
    value_history = portfolio.total_value[timestamp > cutoff]

    if value_history..length > 1:
        start_value = value_history..first
        end_value = value_history..last

        total_return = end_value - start_value
        return_percent = (total_return / start_value) * 100

        return {
            period: days & " days",
            start_value: start_value,
            end_value: end_value,
            total_return: total_return,
            return_percent: return_percent
        }
    else:
        return {error: "Insufficient data for analysis"}

# Test the trading system
create_portfolio("PORTFOLIO_001", 10000)

# Update market data
update_market_price("AAPL", 150.25)
update_market_price("GOOGL", 2750.80)
update_market_price("TSLA", 245.60)

# Place some orders
place_order("PORTFOLIO_001", "AAPL", 10, "BUY")
place_order("PORTFOLIO_001", "GOOGL", 2, "BUY")

# Simulate price changes
update_market_price("AAPL", 155.30)  # Price increase
update_market_price("GOOGL", 2680.40)  # Price decrease

# Analyze performance
performance = analyze_portfolio_performance("PORTFOLIO_001", 1)
output("Portfolio Performance:")
output("  Return: $" & performance.total_return & " (" & performance.return_percent & "%)")
```

---

## 11. Performance and Optimization

### Query Optimization

```mbl
# Efficient temporal queries
# Good: Specific time ranges
recent_data = my.metrics.cpu_usage[>@2026-03-09]

# Less efficient: Unbounded queries
all_data = my.metrics.cpu_usage  # Loads entire history

# Use projections for large datasets
procedure get_daily_summaries(metric_path, days):
    summaries = []

    for day in 0..days:
        day_start = @2026-03-10 - (day * 86400)
        day_end = day_start + 86399

        day_data = metric_path[>day_start, <day_end]
        if day_data..length > 0:
            summaries..append({
                date: day_start,
                count: day_data..length,
                avg: day_data..sum / day_data..length,
                min: day_data..min,
                max: day_data..max
            })

    return summaries
```

### Caching Strategies

```mbl
# Implement application-level caching
cache_ttl = 300  # 5 minutes

procedure get_cached_result(cache_key, compute_function):
    cache_entry = my.cache[cache_key]

    if cache_entry and (@now - cache_entry.timestamp) < cache_ttl:
        return cache_entry.value

    # Compute fresh result
    result = compute_function()

    # Cache it
    my.cache[cache_key].value = result
    my.cache[cache_key].timestamp = @now

    return result

# Usage
expensive_result = get_cached_result("daily_report", procedure():
    # Expensive computation here
    return calculate_daily_metrics()
)
```

### Batch Processing

```mbl
# Batch multiple operations for efficiency
procedure batch_sensor_ingestion(sensor_data_batch):
    # Process multiple sensor readings at once
    for reading in sensor_data_batch:
        my.iot.sensor_data[reading.device_id][reading.sensor_type] = {
            value: reading.value,
            timestamp: reading.timestamp,
            batch_id: reading.batch_id
        }

    output("Processed batch of " & sensor_data_batch..length & " readings")

# Use batch processing
sensor_batch = [
    {device_id: "TEMP_001", sensor_type: "temperature", value: 72.1, timestamp: @now, batch_id: "BATCH_001"},
    {device_id: "TEMP_002", sensor_type: "temperature", value: 74.3, timestamp: @now, batch_id: "BATCH_001"},
    {device_id: "HUMID_001", sensor_type: "humidity", value: 48.7, timestamp: @now, batch_id: "BATCH_001"}
]

batch_sensor_ingestion(sensor_batch)
```

### Memory Management

```mbl
# Implement data archival for old data
procedure archive_old_data(path, days_to_keep):
    cutoff = @now - (days_to_keep * 86400)

    # Move old data to archive
    old_data = path[<cutoff]

    if old_data..length > 0:
        archive_key = "archive_" & @now..format("YYYY-MM-DD")
        my.archive[archive_key] = old_data

        output("Archived " & old_data..length & " old records")

    # Note: Actual deletion would require special privileges
    # This is more of a logical archival

# Cleanup caches periodically
procedure cleanup_cache():
    cache_ttl = 3600  # 1 hour
    cutoff = @now - cache_ttl

    expired_keys = []
    for key in my.cache:
        if my.cache[key].timestamp < cutoff:
            expired_keys..append(key)

    output("Cleaned up " & expired_keys..length & " expired cache entries")
```

---

## 12. Troubleshooting

### Common Issues and Solutions

#### Issue 1: Watcher Not Triggering

```mbl
# Problem: Watcher doesn't seem to execute
watch my_watcher(my.data.value):
    output("Value changed to: " & my.data.value)

# Solution: Check if the watcher is actually registered
procedure debug_watchers():
    watchers = world.cluster.watchers
    for watcher in watchers:
        output("Watcher: " & watcher.name & " monitors: " & watcher.targets)

# Also ensure the data path exists and is being modified
my.data.value = "initial"
my.data.value = "changed"  # This should trigger the watcher
```

#### Issue 2: Permission Denied

```mbl
# Problem: Cannot read/write certain data
# Solution: Check your permissions and stamps
procedure debug_permissions(path):
    # Check your current identity and stamps
    output("Identity: " & ~.@identity)
    output("Stamps: " & ~.stamp)

    # Check the path's permission requirements
    read_perm = path.@read
    write_perm = path.@write

    output("Read permission required: " & read_perm)
    output("Write permission required: " & write_perm)

# Example usage
debug_permissions(my.sensitive_data)
```

#### Issue 3: Unknown Values

```mbl
# Problem: Operations returning Unknown values
result = my.data.number / my.data.other_number

if result ?= Unknown:
    output("Division failed: " & result..reason)

    # Debug the operands
    output("First operand: " & my.data.number & " (type: " & type(my.data.number) & ")")
    output("Second operand: " & my.data.other_number & " (type: " & type(my.data.other_number) & ")")

# Solution: Always validate inputs
procedure safe_calculation(a, b, operation):
    # Type checking
    if type(a) != "Number" or type(b) != "Number":
        return Unknown("Invalid operand types")

    # Operation-specific checks
    if operation ?= "divide" and b ?= 0:
        return Unknown("Division by zero")

    # Perform operation
    if operation ?= "add":
        return a + b
    elif operation ?= "divide":
        return a / b
    else:
        return Unknown("Unknown operation: " & operation)
```

#### Issue 4: Performance Problems

```mbl
# Problem: Slow queries
# Solution: Add timing and optimization

procedure timed_query(query_name, query_function):
    start_time = @now
    result = query_function()
    end_time = @now

    duration = end_time - start_time
    output("Query '" & query_name & "' took " & duration & " seconds")

    if duration > 5:  # Slow query threshold
        my.performance.slow_queries..append({
            name: query_name,
            duration: duration,
            timestamp: @now
        })

    return result

# Usage
result = timed_query("complex_aggregation", procedure():
    return my.large_dataset[complex_condition]..aggregate()
)

# Monitor slow queries
procedure show_slow_queries():
    output("=== Slow Queries ===")
    for query in my.performance.slow_queries:
        output(query.timestamp & ": " & query.name & " (" & query.duration & "s)")
```

### Diagnostic Procedures

```mbl
# Complete system health check
procedure system_health_check():
    output("=== AmorphDB System Health Check ===")

    # Check basic connectivity
    output("1. Connectivity: " & (world.cluster ? "Connected" : "Disconnected"))

    # Check node status
    if world.cluster.nodes:
        output("2. Nodes: " & world.cluster.nodes..length & " active")
        for node in world.cluster.nodes:
            output("   - " & node.id & ": " & node.status)

    # Check zone distribution
    if world.cluster.zones:
        output("3. Zones: " & world.cluster.zones..length & " total")

    # Check recent errors
    recent_errors = my.system.errors[timestamp > (@now - 3600)]  # Last hour
    output("4. Recent Errors: " & recent_errors..length)

    # Check memory usage
    if world.cluster.metrics:
        output("5. Memory Usage: " & world.cluster.metrics.memory_usage & "%")

    # Check watcher health
    watchers = world.cluster.watchers
    active_watchers = watchers[status = "active"]..length
    output("6. Watchers: " & active_watchers & "/" & watchers..length & " active")

system_health_check()

# Data consistency check
procedure check_data_consistency(path):
    output("=== Data Consistency Check for " & path & " ===")

    # Check if data exists
    if not path:
        output("ERROR: Path does not exist")
        return

    # Check timestamps
    current = path
    previous_time = @now + 1  # Future time to start with

    inconsistencies = 0
    while current:
        if current.@timestamp > previous_time:
            output("WARNING: Timestamp inconsistency detected")
            inconsistencies = inconsistencies + 1

        previous_time = current.@timestamp
        current = current.@previous

    if inconsistencies > 0:
        output("Found " & inconsistencies & " timestamp inconsistencies")
    else:
        output("No inconsistencies found")

# Check specific data path
check_data_consistency(my.important_data)
```

### Best Practices Summary

1. **Use specific temporal ranges** instead of querying all history
2. **Implement caching** for expensive operations
3. **Batch operations** when possible
4. **Monitor performance** with timing functions
5. **Handle Unknown values** gracefully
6. **Set appropriate permissions** early in development
7. **Use watchers judiciously** - too many can impact performance
8. **Regular health checks** to catch issues early

---

## Conclusion

You've now learned the fundamentals of AmorphDB and MBL programming! This tutorial covered:

- **Basic MBL syntax** and data types
- **Temporal data operations** and queries
- **Hierarchical data organization**
- **Reactive programming** with watchers
- **Object-oriented features** with templates and inheritance
- **Distributed system** concepts and operations
- **Security and permissions** management
- **Advanced features** and optimization techniques
- **Real-world applications** and best practices

### Next Steps

1. **Practice** with the exercises throughout this tutorial
2. **Build** a small application using the patterns you've learned
3. **Explore** the advanced features for your specific use case
4. **Join** the AmorphDB community for support and sharing
5. **Read** the full documentation for deeper technical details

### Resources

- **[System Design Document](amorphdb_design.md)**: Complete technical specification
- **[White Paper](AmorphDB_White_Paper.md)**: Business overview and competitive analysis
- **[API Reference](api_reference.md)**: Complete MBL language reference
- **[Production Guide](production_guide.md)**: Deployment and operations guide

Welcome to the future of temporal database programming with AmorphDB!