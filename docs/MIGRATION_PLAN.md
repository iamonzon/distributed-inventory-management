## Migration from Legacy System
### The Legacy System
```
- Monolithic backend
- 15-minute batch sync
- Each store has local DB
- Legacy frontend
```

### New System
```
- Service A + Service B microservices
- Event-driven via Redis
- CAS-based consistency
```

---

### Strategy: Strangler Fig Pattern

1. **Deploy Service A alongside legacy monolith**
   - Dual writes: write to both old and new system
   - Validate data consistency
   
2. **Deploy Service B to 1 pilot store**
   - Monitor for issues
   - Compare checkout success rates
   
3. **Gradual rollout to all stores (10% per week)**
   - Automated rollback if error rate > 0.1%
   
4. **Decommission legacy system after 100% migration**

### Rollback Plan
If critical issues discovered:
- All stores can instantly fall back to legacy system
- No data loss (dual-write period covers this)
- Maximum downtime: 5 minutes