import React from 'react';

const MealBlockedBanner = ({ reason, onOverride }) => {
    if (!reason) return null;

    return (
        <div className="glass-panel p-4 mb-6" style={{ background: '#fef2f2', border: '1px solid #fee2e2', borderLeft: '4px solid #ef4444' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', color: '#b91c1c' }}>
                <span style={{ fontSize: '1.2rem' }}>⚠️</span>
                <div style={{ flex: 1 }}>
                    <strong>Meal Blocked:</strong> {reason}
                </div>
            </div>
            <div style={{ marginTop: '0.75rem', textAlign: 'right' }}>
                <button
                    className="btn"
                    style={{ background: '#b91c1c', color: 'white', fontSize: '0.85rem' }}
                    onClick={onOverride}
                >
                    I understand, log anyway
                </button>
            </div>
        </div>
    );
};

export default MealBlockedBanner;
