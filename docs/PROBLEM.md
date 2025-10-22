# Optimization of a Distributed Inventory Management System

## Objective:

Design and prototype an improvement to an existing inventory management system operating in a distributed environment. The goal is to optimize inventory consistency, reduce stock update latency, and lower operational costs, while ensuring security and observability.

## Context:

Your company maintains an inventory management system for a chain of retail stores. Currently, each store has a local database that periodically (every 15 minutes) synchronizes with a central database. Customers can view available stock online, but inconsistencies and latency in updates have led to user experience issues and lost sales due to stock discrepancies. The current system has a monolithic backend, and the frontend is a legacy web application.

## Requirements:

### Technical Design:

- Propose a distributed architecture that addresses the consistency and latency issues of the current system.
- Design the API for key inventory operations.
- Justify your technical and API design decisions, explaining why they are best suited for this distributed scenario.

## Backend:

- Implement a simplified prototype of the proposed backend services. Use a programming language of your choice.
- Simulate data persistence using local JSON/CSV files or an in-memory database (e.g., SQLite, H2 Database) to represent the inventory. A real database is not required.
- Implement basic fault tolerance mechanisms as you deem necessary.
- Include logic to handle stock updates in a concurrent environment, prioritizing consistency over availability if necessary (or vice versa), justifying your choice.

## Non-functional requirements:

Special consideration will be given to good practices in error handling, documentation, testing, and any other relevant non-functional aspects you choose to demonstrate

## Tool Usage

**Allowed Tools:** You may use and are encouraged to use GenAI tools, agentic IDEs, and other code assistance tools to help generate ideas or code.

## Documentation & strategic overview

Please include a brief README o Diagram (optional) that explains your API design, main endpoints, setup instructions, and any key architectural decisions you made during development.

## Technical strategy:

Detail the chosen technology stack for backend Explain how GenAI and modern development tools are integrated to improve efficiency