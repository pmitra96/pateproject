import React, { useState } from 'react';

const MacroTargetModal = ({ goalId, currentTargets, onClose, onSave }) => {
    const [form, setForm] = useState(currentTargets || {
        daily_calorie_target: 2000,
        daily_protein_target: 150,
        daily_fat_target: 60,
        daily_carbs_target: 200,
        damage_control_floor_calories: 300,
        macro_priority: ["protein", "calories", "fat", "carbs"]
    });

    // Calculator State
    const [showCalculator, setShowCalculator] = useState(false);
    const [calcForm, setCalcForm] = useState({
        weight: 80,
        height: 175,
        age: 30,
        gender: 'male',
        activity: 'sedentary',
        goal_type: 'lose' // lose, maintain, gain
    });

    const handleChange = (field, val) => {
        setForm(prev => ({ ...prev, [field]: parseFloat(val) || 0 }));
    };

    const handleCalculate = () => {
        // Mifflin-St Jeor Equation
        let bmr = (10 * calcForm.weight) + (6.25 * calcForm.height) - (5 * calcForm.age);
        if (calcForm.gender === 'male') bmr += 5;
        else bmr -= 161;

        // TDEE Multipliers
        const activityMultipliers = {
            sedentary: 1.2,
            light: 1.375,
            moderate: 1.55,
            active: 1.725
        };
        const tdee = bmr * (activityMultipliers[calcForm.activity] || 1.2);

        // Goal Adjustment
        let targetCalories = tdee;
        if (calcForm.goal_type === 'lose') targetCalories -= 500; // -0.5kg/week
        else if (calcForm.goal_type === 'gain') targetCalories += 300;

        // Set Macros (Balanced: 30P/35F/35C roughly or 2g/kg protein)
        const protein = Math.min(calcForm.weight * 2.2, targetCalories * 0.3 / 4); // Cap protein at 2.2g/kg or 30%
        const fat = (targetCalories * 0.3) / 9; // 30% fat
        const carbs = (targetCalories - (protein * 4) - (fat * 9)) / 4; // Remainder carbs

        setForm({
            ...form,
            daily_calorie_target: Math.round(targetCalories),
            daily_protein_target: Math.round(protein),
            daily_fat_target: Math.round(fat),
            daily_carbs_target: Math.round(carbs),
            damage_control_floor_calories: 300
        });
        setShowCalculator(false);
    };

    const handleSubmit = (e) => {
        e.preventDefault();
        onSave(goalId, form);
    };

    return (
        <div className="modal-overlay" style={{
            position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', justifyContent: 'center', alignItems: 'center', zIndex: 1000
        }}>
            <div className="glass-panel p-6" style={{ background: 'white', maxWidth: '500px', width: '90%', maxHeight: '90vh', overflowY: 'auto' }}>
                <div className="flex justify-between items-center mb-4">
                    <h2 style={{ margin: 0 }}>Set Goal Targets</h2>
                    <button className="btn btn-secondary" style={{ fontSize: '0.8rem', padding: '0.3rem 0.6rem' }} onClick={() => setShowCalculator(!showCalculator)}>
                        {showCalculator ? 'Hide Calculator' : '🧮 Auto-Calculate'}
                    </button>
                </div>

                {showCalculator && (
                    <div className="p-4 mb-4 bg-gray-50 rounded border border-gray-200" style={{ background: '#f9fafb' }}>
                        <h4 className="mb-3">Macro Calculator</h4>
                        <div className="grid grid-cols-2 gap-3 mb-3">
                            <div>
                                <label className="text-xs">Weight (kg)</label>
                                <input type="number" value={calcForm.weight} onChange={e => setCalcForm({ ...calcForm, weight: parseFloat(e.target.value) })} className="w-full p-1 border rounded" />
                            </div>
                            <div>
                                <label className="text-xs">Height (cm)</label>
                                <input type="number" value={calcForm.height} onChange={e => setCalcForm({ ...calcForm, height: parseFloat(e.target.value) })} className="w-full p-1 border rounded" />
                            </div>
                            <div>
                                <label className="text-xs">Age</label>
                                <input type="number" value={calcForm.age} onChange={e => setCalcForm({ ...calcForm, age: parseFloat(e.target.value) })} className="w-full p-1 border rounded" />
                            </div>
                            <div>
                                <label className="text-xs">Gender</label>
                                <select value={calcForm.gender} onChange={e => setCalcForm({ ...calcForm, gender: e.target.value })} className="w-full p-1 border rounded">
                                    <option value="male">Male</option>
                                    <option value="female">Female</option>
                                </select>
                            </div>
                            <div className="col-span-2">
                                <label className="text-xs">Activity</label>
                                <select value={calcForm.activity} onChange={e => setCalcForm({ ...calcForm, activity: e.target.value })} className="w-full p-1 border rounded">
                                    <option value="sedentary">Sedentary (Office Job)</option>
                                    <option value="light">Lightly Active (1-3 days/week)</option>
                                    <option value="moderate">Moderately Active (3-5 days/week)</option>
                                    <option value="active">Very Active (6-7 days/week)</option>
                                </select>
                            </div>
                            <div className="col-span-2">
                                <label className="text-xs">Goal</label>
                                <select value={calcForm.goal_type} onChange={e => setCalcForm({ ...calcForm, goal_type: e.target.value })} className="w-full p-1 border rounded">
                                    <option value="lose">Lose Weight (-0.5kg/week)</option>
                                    <option value="maintain">Maintain Weight</option>
                                    <option value="gain">Gain Muscle (+0.3kg/week)</option>
                                </select>
                            </div>
                        </div>
                        <button type="button" className="btn w-full" style={{ background: 'var(--accent-color)', color: 'white' }} onClick={handleCalculate}>
                            Calculate & Fill
                        </button>
                    </div>
                )}

                <form onSubmit={handleSubmit}>
                    <div className="mb-4">
                        <label>Daily Calories</label>
                        <input type="number"
                            value={form.daily_calorie_target}
                            onChange={e => handleChange('daily_calorie_target', e.target.value)}
                            className="w-full p-2 border rounded" />
                    </div>

                    <div className="grid grid-cols-3 gap-4 mb-4">
                        <div>
                            <label>Protein (g)</label>
                            <input type="number" value={form.daily_protein_target} onChange={e => handleChange('daily_protein_target', e.target.value)} className="w-full p-2 border rounded" />
                        </div>
                        <div>
                            <label>Fat (g)</label>
                            <input type="number" value={form.daily_fat_target} onChange={e => handleChange('daily_fat_target', e.target.value)} className="w-full p-2 border rounded" />
                        </div>
                        <div>
                            <label>Carbs (g)</label>
                            <input type="number" value={form.daily_carbs_target} onChange={e => handleChange('daily_carbs_target', e.target.value)} className="w-full p-2 border rounded" />
                        </div>
                    </div>

                    <div className="mb-4">
                        <label>Damage Control Floor (Calories)</label>
                        <div className="text-secondary text-sm mb-1">Minimum safe calories to intake if over budget</div>
                        <input type="number" value={form.damage_control_floor_calories} onChange={e => handleChange('damage_control_floor_calories', e.target.value)} className="w-full p-2 border rounded" />
                    </div>

                    <div className="flex justify-end gap-2">
                        <button type="button" className="btn btn-secondary" onClick={onClose}>Cancel</button>
                        <button type="submit" className="btn btn-primary">Save Targets</button>
                    </div>
                </form>
            </div>
        </div>
    );
};

export default MacroTargetModal;
