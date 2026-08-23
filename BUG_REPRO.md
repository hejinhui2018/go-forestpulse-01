# Operational reproduction context

An intermittently connected station may submit a continuous batch while the storage device is temporarily unavailable. Operators should retain the service error and retry the same batch after storage recovers. A successful recovery must make every sequence in the batch readable and report a cursor matching the last readable sequence.

Useful observations are the batch identifier, station identifier, submitted sequence range, storage error, cursor returned after retry, and the sequences returned by the readings endpoint. This document intentionally describes only the operational behavior and contains no implementation guidance.

