import React from 'react';

const RemainingDayPanel = ({ state }) => {
    if (!state) return null;

    const {
        remaining_calories,
        remaining_protein,
        remaining_fat,
        remaining_carbs,
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
            </div>

            <div className="grid grid-cols-3 gap-4">
                <MacroBar label="Protein" val={remaining_protein} color="#3b82f6" />
                <MacroBar label="Carbs" val={remaining_carbs} color="#10b981" />
                <MacroBar label="Fat" val={remaining_fat} color="#f59e0b" />
            </div>
        </div>
    );
};

const MacroBar = ({ label, val, color }) => (
    <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', marginBottom: '0.25rem' }}>
            <span>{label}</span>
            <span style={{ fontWeight: 600 }}>{Math.round(val)}g</span>
        </div>
        <div style={{ height: '6px', background: '#e5e7eb', borderRadius: '3px', overflow: 'hidden' }}>
            <div style={{
                height: '100%',
                background: color,
                width: val > 0 ? '100%' : '0%'
            }} />
        </div>
    </div>
);

export default RemainingDayPanel;
