import React from 'react';

const NextActionCard = ({ action, onLog, onOverride, onDone }) => {
    if (!action) return null;

    return (
        <div className="glass-panel p-6 mb-6" style={{ background: 'linear-gradient(to right, #f8fafc, #fff)' }}>
            <h3 style={{ marginTop: 0 }}>Next Step</h3>

            {action.type === 'stop_eating' ? (
                <div style={{ color: '#ef4444', fontWeight: 500 }}>
                    🛑 {action.message}
                    <div style={{ marginTop: '1rem' }}>
                        <button className="btn" onClick={onDone}>Done for Today</button>
                    </div>
                </div>
            ) : (
                <div>
                    <div className="mb-4">
                        Suggestion: <strong>{action.meal?.name || 'Balanced Meal'}</strong>
                        <div className="text-secondary" style={{ fontSize: '0.9rem' }}>
                            ~{action.meal?.calories} kcal
                        </div>
                    </div>

                    <div style={{ display: 'flex', gap: '0.5rem' }}>
                        <button className="btn btn-primary" onClick={() => onLog(action.meal)}>Log this Meal</button>
                        <button className="btn btn-secondary" onClick={onOverride}>I ate something else</button>
                    </div>
                </div>
            )}
        </div>
    );
};

export default NextActionCard;
