import React from 'react';

const RemainingDayPanel = ({ state }) => {
    if (!state) return null;

    const {
        remaining_calories,
        remaining_protein,
        remaining_fat,
        remaining_carbs,
        target_calories,  // New field
        target_protein,   // New field
        target_fat,       // New field
        target_carbs,     // New field
        control_mode,
        meals_remaining
    } = state;

    const getBadgeColor = (mode) => {
        switch (mode) {
            case 'NORMAL': return '#10b981'; // Green
            case 'TIGHT': return '#f59e0b'; // Yellow/Orange
            case 'DAMAGE_CONTROL': return '#ef4444'; // Red
            default: return '#6b7280';
        }
    };

    const getBadgeText = (mode) => {
        switch (mode) {
            case 'NORMAL': return 'Normal Mode';
            case 'TIGHT': return 'Tight Budget';
            case 'DAMAGE_CONTROL': return 'Damage Control';
            default: return mode;
        }
    };

    // Helper to render macro bars
    // target might be 0 if old state, handle gracefully
    const renderMacro = (label, remaining, target, color) => {
        const safeTarget = target || 1; // avoid div by zero
        const consumed = safeTarget - remaining;
        const percentConsumed = Math.min(100, Math.max(0, (consumed / safeTarget) * 100));

        return (
            <MacroBar
                label={label}
                remaining={remaining}
                target={target}
                percentConsumed={percentConsumed}
                color={color}
            />
        );
    };

    return (
        <div className="glass-panel p-4 mb-6" style={{ borderLeft: `4px solid ${getBadgeColor(control_mode)}` }}>
            <div className="flex-between mb-4">
                <div>
                    <h3 style={{ margin: 0, fontSize: '1.1rem' }}>Today's Budget</h3>
                    <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                        {meals_remaining} meals remaining
                    </div>
                </div>
                <span className="badge" style={{ background: getBadgeColor(control_mode), color: 'white' }}>
                    {getBadgeText(control_mode)}
                </span>
            </div>

            <div style={{ marginBottom: '1rem' }}>
                <div style={{ fontSize: '2rem', fontWeight: 700, color: remaining_calories < 0 ? '#ef4444' : 'var(--text-primary)' }}>
                    {Math.round(remaining_calories)} <span style={{ fontSize: '1rem', fontWeight: 400, color: 'var(--text-secondary)' }}>kcal left</span>
                </div>
                {target_calories > 0 && (
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '0.2rem' }}>
                        {Math.round(((target_calories - remaining_calories) / target_calories) * 100)}% of daily budget used
                    </div>
                )}
            </div>

            <div className="grid grid-cols-3 gap-4">
                {renderMacro("Protein", remaining_protein, target_protein, "#3b82f6")}
                {renderMacro("Carbs", remaining_carbs, target_carbs, "#10b981")}
                {renderMacro("Fat", remaining_fat, target_fat, "#f59e0b")}
            </div>
        </div>
    );
};

const MacroBar = ({ label, remaining, target, percentConsumed, color }) => (
    <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', marginBottom: '0.25rem' }}>
            <span>{label}</span>
            <span style={{ fontWeight: 600 }}>{Math.round(remaining)}g left</span>
        </div>
        <div style={{ height: '6px', background: '#e5e7eb', borderRadius: '3px', overflow: 'hidden' }}>
            <div style={{
                height: '100%',
                background: color,
                width: `${percentConsumed}%`,
                transition: 'width 0.3s ease'
            }} />
        </div>
        <div style={{ textAlign: 'right', fontSize: '0.7rem', color: 'var(--text-secondary)', marginTop: '2px' }}>
            {Math.round(percentConsumed)}% used
        </div>
    </div>
);

export default RemainingDayPanel;
