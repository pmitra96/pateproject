import React, { useState } from 'react';

const CanIEatModal = ({ isOpen, onClose, checkResult, onLog, onLogSmall, foodName, onCheck, onReset }) => {
    const [inputQuery, setInputQuery] = useState('');
    const [isLoading, setIsLoading] = useState(false);

    if (!isOpen) return null;

    // INPUT STATE: If no result yet, show input form
    if (!checkResult) {
        const handleSubmit = async (e) => {
            e.preventDefault();
            if (!inputQuery.trim()) return;
            setIsLoading(true);
            try {
                await onCheck(inputQuery);
                setIsLoading(false);
                // Parent will update checkResult, causing re-render to Result state
            } catch (err) {
                console.error(err);
                setIsLoading(false);
            }
        };

        return (
            <div style={{
                position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
                backgroundColor: 'rgba(0, 0, 0, 0.5)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000
            }}>
                <div style={{
                    backgroundColor: 'white', padding: '2rem', borderRadius: '24px',
                    width: '90%', maxWidth: '400px',
                    boxShadow: '0 10px 25px rgba(0,0,0,0.1)'
                }}>
                    <h2 style={{ marginTop: 0, marginBottom: '0.5rem' }}>❓ Can I Eat This?</h2>
                    <p style={{ color: '#666', marginBottom: '1.5rem' }}>
                        Get advice on if a food fits your daily plan.
                    </p>
                    <form onSubmit={handleSubmit}>
                        <input
                            type="text"
                            autoFocus
                            placeholder="e.g., Slice of Pizza, Apple, Cake"
                            value={inputQuery}
                            onChange={(e) => setInputQuery(e.target.value)}
                            style={{
                                width: '100%', padding: '1rem', borderRadius: '12px',
                                border: '1px solid #ddd', fontSize: '1.1rem', marginBottom: '1rem',
                                outline: 'none'
                            }}
                        />
                        <div style={{ display: 'flex', gap: '0.5rem' }}>
                            <button
                                type="button"
                                onClick={onClose}
                                className="btn btn-secondary"
                                style={{ flex: 1, padding: '1rem', borderRadius: '12px' }}
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                className="btn"
                                disabled={isLoading || !inputQuery.trim()}
                                style={{
                                    flex: 1, padding: '1rem', borderRadius: '12px',
                                    background: 'linear-gradient(135deg, #10b981 0%, #059669 100%)',
                                    color: 'white', fontWeight: 'bold'
                                }}
                            >
                                {isLoading ? 'Checking...' : 'Get Advice'}
                            </button>
                        </div>
                    </form>
                </div>
            </div>
        );
    }

    // Handle new API structure or fallback
    const status = checkResult.permission_result ? checkResult.permission_result.status : checkResult.status;
    const reason = checkResult.permission_result ? checkResult.permission_result.reason : checkResult.reason;
    const food = checkResult.food;
    const currentState = checkResult.current_state;
    const simulatedState = checkResult.simulated_state;

    // Configuration for different statuses
    const config = {
        ALLOW: {
            color: '#22c55e', // Green
            bgColor: '#dcfce7',
            borderColor: '#22c55e',
            icon: '✅',
            title: 'Yes, you can eat this.',
            subtext: reason || 'Fits within your remaining budget.',
            primaryAction: { label: 'Log this', handler: onLog },
            secondaryAction: { label: 'Check Another', handler: () => { setInputQuery(''); onReset ? onReset() : onClose(); } }
        },
        ALLOW_WITH_CONSTRAINT: {
            color: '#f59e0b', // Amber
            bgColor: '#fef3c7',
            borderColor: '#f59e0b',
            icon: '⚠️',
            title: 'Yes, but be careful.',
            subtext: reason || 'This takes up a significant portion of your remaining budget.',
            primaryAction: { label: 'Log (Small Portion)', handler: onLogSmall },
            secondaryAction: { label: 'Log (Full)', handler: onLog },
            tertiaryAction: { label: 'Cancel', handler: onClose }
        },
        BLOCK: {
            color: '#ef4444', // Red
            bgColor: '#fee2e2',
            borderColor: '#ef4444',
            icon: '❌',
            title: 'No — this doesn’t fit.',
            subtext: reason || 'Exceeds remaining daily limits.',
            primaryAction: { label: 'Log Anyway (Override)', handler: onLog, danger: true },
            secondaryAction: { label: 'OK', handler: onClose }
        }
    };

    const currentConfig = config[status] || config.BLOCK;

    const renderProgressBar = (label, currentRem, simRem, target, unit = 'g') => {
        if (target <= 0) return null; // No target set

        const used = target - currentRem;
        const newCost = currentRem - simRem;
        // If currentRem is negative, we are already over. 
        // We want to show the bar relative to Target.

        const usedPct = Math.min(100, Math.max(0, (used / target) * 100));
        const costPct = Math.min(100 - usedPct, Math.max(0, (newCost / target) * 100));

        // Colors
        let barColor = '#e5e7eb'; // gray-200
        let fillCurrent = '#9ca3af'; // gray-400
        let fillNew = currentConfig.color; // Contextual color

        const isOver = simRem < 0;

        return (
            <div style={{ newBottom: '0.75rem', fontSize: '0.85rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.25rem' }}>
                    <span style={{ fontWeight: 600, color: '#4b5563' }}>{label}</span>
                    <span style={{ color: isOver ? '#ef4444' : '#4b5563' }}>
                        {Math.round(currentRem)} ➝ <strong>{Math.round(simRem)}</strong> {unit} left
                    </span>
                </div>
                <div style={{
                    height: '8px',
                    width: '100%',
                    backgroundColor: '#f3f4f6',
                    borderRadius: '4px',
                    overflow: 'hidden',
                    display: 'flex'
                }}>
                    <div style={{ width: `${usedPct}%`, background: fillCurrent }}></div>
                    <div style={{ width: `${costPct}%`, background: fillNew }}></div>
                </div>
            </div>
        );
    };

    return (
        <div style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            background: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000
        }}>
            <div style={{
                background: 'white',
                borderRadius: '24px',
                padding: '2rem',
                maxWidth: '400px',
                width: '90%',
                display: 'flex',
                flexDirection: 'column',
                maxHeight: '90vh',
                overflowY: 'auto'
            }}>
                <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: '3rem', marginBottom: '0.5rem' }}>{currentConfig.icon}</div>
                    <h2 style={{ color: currentConfig.color, margin: '0 0 0.5rem 0', fontSize: '1.5rem' }}>{currentConfig.title}</h2>
                    <p style={{ color: '#666', fontSize: '1rem', marginBottom: '1.5rem', lineHeight: '1.4' }}>{currentConfig.subtext}</p>
                </div>

                {/* Simulation Panel */}
                {currentState && simulatedState && (
                    <div style={{
                        background: '#f9fafb',
                        border: '1px solid #e5e7eb',
                        borderRadius: '16px',
                        padding: '1rem',
                        marginBottom: '1.5rem'
                    }}>
                        <h4 style={{ margin: '0 0 1rem 0', fontSize: '0.9rem', textTransform: 'uppercase', color: '#6b7280', letterSpacing: '0.05em' }}>
                            Projected Impact
                        </h4>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                            {renderProgressBar('Calories', currentState.remaining_calories, simulatedState.remaining_calories, currentState.target_calories, 'kcal')}
                            {renderProgressBar('Protein', currentState.remaining_protein, simulatedState.remaining_protein, currentState.target_protein, 'g')}
                            {renderProgressBar('Fat', currentState.remaining_fat, simulatedState.remaining_fat, currentState.target_fat, 'g')}
                            {renderProgressBar('Carbs', currentState.remaining_carbs, simulatedState.remaining_carbs, currentState.target_carbs, 'g')}
                        </div>
                    </div>
                )}

                {/* Estimated Nutrition Display (Compact) */}
                {food && (
                    <div style={{
                        display: 'flex', justifyContent: 'space-between',
                        fontSize: '0.85rem', color: '#6b7280',
                        marginBottom: '1.5rem', padding: '0 0.5rem'
                    }}>
                        <span>{food.name || foodName}</span>
                        <span>{Math.round(food.calories)} kcal</span>
                    </div>
                )}

                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                    <button
                        onClick={currentConfig.primaryAction.handler}
                        style={{
                            background: currentConfig.primaryAction.danger ? 'transparent' : currentConfig.color,
                            color: currentConfig.primaryAction.danger ? currentConfig.color : 'white',
                            border: currentConfig.primaryAction.danger ? `2px solid ${currentConfig.color}` : 'none',
                            padding: '1rem',
                            borderRadius: '12px',
                            fontSize: '1.1rem',
                            fontWeight: '600',
                            cursor: 'pointer',
                            width: '100%'
                        }}
                    >
                        {currentConfig.primaryAction.label}
                    </button>

                    {currentConfig.secondaryAction && (
                        <button
                            onClick={currentConfig.secondaryAction.handler}
                            style={{
                                background: 'transparent',
                                color: '#666',
                                border: '2px solid #eee',
                                padding: '0.8rem',
                                borderRadius: '12px',
                                fontSize: '1rem',
                                fontWeight: '600',
                                cursor: 'pointer',
                                width: '100%'
                            }}
                        >
                            {currentConfig.secondaryAction.label}
                        </button>
                    )}
                    {currentConfig.tertiaryAction && (
                        <button
                            onClick={currentConfig.tertiaryAction.handler}
                            style={{
                                background: 'transparent',
                                color: '#999',
                                border: 'none',
                                padding: '0.5rem',
                                fontSize: '0.9rem',
                                cursor: 'pointer',
                                textDecoration: 'underline'
                            }}
                        >
                            {currentConfig.tertiaryAction.label}
                        </button>
                    )}
                </div>
            </div>
        </div>
    );
};

export default CanIEatModal;
