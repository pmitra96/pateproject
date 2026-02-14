import { useState, useEffect } from 'react';
import { GoogleLogin } from '@react-oauth/google';
import { jwtDecode } from 'jwt-decode';
import {
  fetchPantry,
  updatePantryItem,
  deletePantryItem,
  bulkDeletePantryItems,
  extractItems,
  ingestOrder,
  suggestMealPersonalized,
  fetchGoals,
  createGoal,
  deleteGoal,
  logMeal,
  fetchMealHistory,
  deleteMealLog,
  sendChatMessage,
  saveConversation,
  fetchConversations,
  fetchUserPreferences,
  updateUserPreferences,
  addPantryItem,
  fetchRemainingDayState,
  setGoalMacroTargets,
  checkFoodPermission,
  // validateMeal,

} from './api';

import RemainingDayPanel from './components/RemainingDayPanel';

import MealBlockedBanner from './components/MealBlockedBanner';
import MacroTargetModal from './components/MacroTargetModal';
import CanIEatModal from './components/CanIEatModal';

const getDefaultMealType = () => {
  const hour = new Date().getHours();
  if (hour >= 5 && hour < 12) return 'Breakfast';
  if (hour >= 12 && hour < 17) return 'Lunch';
  if (hour >= 17 && hour < 21) return 'Dinner';
  return 'Snack';
};

function App() {
  const [pantry, setPantry] = useState([]);
  const [orders, setOrders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('home');
  const [searchTerm, setSearchTerm] = useState('');

  const [editingId, setEditingId] = useState(null);
  const [editValue, setEditValue] = useState('');
  const [deletingItem, setDeletingItem] = useState(null);
  const [selectedIds, setSelectedIds] = useState(new Set());
  const [showBulkDeleteConfirm, setShowBulkDeleteConfirm] = useState(false);
  const [user, setUser] = useState(null);
  const [extractionResult, setExtractionResult] = useState(null);
  const [isExtracting, setIsExtracting] = useState(false);
  const [mealSuggestions, setMealSuggestions] = useState(null);
  const [isLoadingSuggestions, setIsLoadingSuggestions] = useState(false);
  const [goals, setGoals] = useState([]);
  const [showAddGoal, setShowAddGoal] = useState(false);
  const [newGoalTitle, setNewGoalTitle] = useState('');
  const [newGoalDescription, setNewGoalDescription] = useState('');
  const [viewingNutritionItem, setViewingNutritionItem] = useState(null);
  const [selectedMeal, setSelectedMeal] = useState(null);
  const [selectedMealType, setSelectedMealType] = useState(getDefaultMealType()); // Smart default based on time
  const [isLoggingMeal, setIsLoggingMeal] = useState(false);
  const [mealHistory, setMealHistory] = useState([]);


  // Remaining Day Control State
  const [remainingDayState, setRemainingDayState] = useState(null);

  const [showTargetModal, setShowTargetModal] = useState(false);
  const [targetModalGoalId, setTargetModalGoalId] = useState(null);

  const [currentBlockedReason, setCurrentBlockedReason] = useState(null);
  const [requestOverride, setRequestOverride] = useState(false);
  // Chatbot state
  const [chatOpen, setChatOpen] = useState(false);
  const [chatMessages, setChatMessages] = useState([]);
  const [chatInput, setChatInput] = useState('');
  const [isChatLoading, setIsChatLoading] = useState(false);
  const [pastConversations, setPastConversations] = useState([]);
  const [showConversationHistory, setShowConversationHistory] = useState(false);

  // Can I Eat State
  const [showCanIEatModal, setShowCanIEatModal] = useState(false);
  const [canIEatResult, setCanIEatResult] = useState(null);
  const [pendingMealLog, setPendingMealLog] = useState(null); // To store meal data while modal is open



  // User preferences state
  const [showPreferencesModal, setShowPreferencesModal] = useState(false);
  const [userPreferences, setUserPreferences] = useState({
    country: '',
    state: '',
    city: '',
    preferred_cuisines: []
  });
  const [preferencesForm, setPreferencesForm] = useState({
    country: '',
    state: '',
    city: '',
    preferred_cuisines: []
  });
  const [newCuisine, setNewCuisine] = useState('');

  // Manual item add state
  const [manualItemName, setManualItemName] = useState('');
  const [manualItemQty, setManualItemQty] = useState('');
  const [manualItemUnit, setManualItemUnit] = useState('pcs');
  const [isAddingItem, setIsAddingItem] = useState(false);

  // Meal logging state
  const [mealName, setMealName] = useState('');
  const [mealIngredients, setMealIngredients] = useState([]);
  const [ingredientName, setIngredientName] = useState('');
  const [ingredientQty, setIngredientQty] = useState('1');
  const [ingredientUnit, setIngredientUnit] = useState('pcs');
  const [ingredientSuggestions, setIngredientSuggestions] = useState([]);

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (token) {
      try {
        if (token.includes('.')) {
          const decoded = jwtDecode(token);
          setUser(decoded);
        } else {
          // Fallback for dev IDs like "1"
          setUser({ sub: token, name: `User ${token}` });
        }
      } catch (err) {
        console.error("Invalid token", err);
        // If it's not a JWT and not a simple ID, maybe it's just garbage
        if (token.length > 20) {
          localStorage.removeItem('token');
          setUser(null);
        } else {
          setUser({ sub: token, name: `User ${token}` });
        }
      }
    }
  }, []);

  useEffect(() => {
    if (user) {
      loadData();
      loadPreferences();
    }
  }, [user, activeTab]);
  // Given loadData uses state, it should be wrapped or dep array fixed.
  // For simplicity effectively suppressing by not adding it, as it might cause loops if not careful.
  // But linter complained. Let's just suppress or leave as is if warning. 
  // Actually, let's just leave it as a warning if it doesn't break build, but user might want clean output.
  // Let's wrapping loadData in useCallback is too big of a change.
  // I'll just suffix the line with eslint-disable-line if possible, but reacting to specific lines.
  // Let's just fix the unused vars.

  const loadPreferences = async () => {
    try {
      const prefs = await fetchUserPreferences();
      setUserPreferences(prefs);
      setPreferencesForm(prefs);
    } catch (err) {
      console.error('Failed to load preferences', err);
    }
  };

  const handleSavePreferences = async () => {
    try {
      const updated = await updateUserPreferences(preferencesForm);
      setUserPreferences(updated);
      setShowPreferencesModal(false);
    } catch (err) {
      console.error('Failed to save preferences', err);
    }
  };

  const handleAddCuisine = () => {
    if (newCuisine.trim() && !preferencesForm.preferred_cuisines.includes(newCuisine.trim())) {
      setPreferencesForm(prev => ({
        ...prev,
        preferred_cuisines: [...prev.preferred_cuisines, newCuisine.trim()]
      }));
      setNewCuisine('');
    }
  };

  const handleRemoveCuisine = (cuisine) => {
    setPreferencesForm(prev => ({
      ...prev,
      preferred_cuisines: prev.preferred_cuisines.filter(c => c !== cuisine)
    }));
  };

  // SSE: Listen for real-time nutrition updates
  useEffect(() => {
    if (!user) return;

    const eventSource = new EventSource('http://localhost:8080/sse/nutrition');

    eventSource.addEventListener('nutrition_update', (event) => {
      try {
        const update = JSON.parse(event.data);
        console.log('Received nutrition update:', update);

        // Update pantry items with new nutrition data
        setPantry(prev => prev.map(item => {
          if (item.item.id === update.item_id) {
            return {
              ...item,
              item: {
                ...item.item,
                calories: update.calories,
                protein: update.protein,
                carbs: update.carbs,
                fat: update.fat,
                fiber: update.fiber,
                nutrition_verified: update.verified
              }
            };
          }
          return item;
        }));
      } catch (err) {
        console.error('Failed to parse nutrition update:', err);
      }
    });

    eventSource.addEventListener('connected', () => {
      console.log('SSE connected for nutrition updates');
    });

    eventSource.onerror = (err) => {
      console.error('SSE error:', err);
    };

    return () => {
      eventSource.close();
    };
  }, [user]);


  const loadState = async () => {
    try {
      const state = await fetchRemainingDayState();
      setRemainingDayState(state);
      // Also fetch next action if state exists

    } catch (err) {
      console.error("Failed to load remaining day state", err);
    }
  };

  const loadData = async () => {
    setLoading(true);
    setSelectedIds(new Set());
    try {
      // Load state always if user exists
      loadState();

      if (activeTab === 'home') {
        const data = await fetchPantry();
        setPantry(data);
        const goalsData = await fetchGoals();
        setGoals(goalsData || []);
      } else if (activeTab === 'inventory') {
        const data = await fetchPantry();
        setPantry(data);
        const goalsData = await fetchGoals();
        setGoals(goalsData || []);
      } else if (activeTab === 'goals') {
        const goalsData = await fetchGoals();
        setGoals(goalsData || []);
      } else if (activeTab === 'history') {
        const res = await fetch('http://localhost:8080/orders', {
          headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
        });
        const data = await res.json();
        setOrders(data);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleEdit = (item) => {
    setEditingId(item.id);
    setEditValue(item.effective_quantity);
  };

  const handleSave = async (id) => {
    const val = parseFloat(editValue);
    if (isNaN(val)) return;
    setEditingId(null);
    try {
      const itemToUpdate = pantry.find(p => p.id === id);
      await updatePantryItem(itemToUpdate.item_id, val);
      const data = await fetchPantry();
      setPantry(data);
    } catch (err) {
      console.error("Failed to save", err);
    }
  };

  const handleDelete = (item) => {
    setDeletingItem(item);
  };

  const handleConfirmDelete = async () => {
    if (!deletingItem) return;
    try {
      await deletePantryItem(deletingItem.item_id);
      loadData();
      setDeletingItem(null);
    } catch (err) {
      console.error("Failed to delete", err);
      alert("Failed to delete item");
    }
  };

  const handleSelectItem = (id) => {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelectedIds(next);
  };

  const handleSelectAll = (e) => {
    if (e.target.checked) {
      setSelectedIds(new Set(filteredPantry.map(i => i.item_id)));
    } else {
      setSelectedIds(new Set());
    }
  };

  const handleBulkDelete = () => {
    if (selectedIds.size > 0) {
      setShowBulkDeleteConfirm(true);
    }
  };

  const handleConfirmBulkDelete = async () => {
    try {
      await bulkDeletePantryItems(Array.from(selectedIds));
      loadData();
      setShowBulkDeleteConfirm(false);
    } catch (err) {
      console.error("Failed to bulk delete", err);
      alert("Failed to delete selected items");
    }
  };

  const handleFileUpload = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    setIsExtracting(true);
    try {
      const result = await extractItems(file);
      setExtractionResult(result);
    } catch {
      alert("Failed to extract items from image.");
    } finally {
      setIsExtracting(false);
    }
  };

  const handleConfirmIngest = async () => {
    if (!extractionResult) return;
    try {
      const orderData = {
        user_id: user?.sub || user?.id || "1",
        external_order_id: `INV_${Date.now()}`,
        provider: extractionResult.provider || "unknown",
        order_date: new Date().toISOString(),
        items: extractionResult.items.map(item => ({
          raw_name: item.name,
          quantity: item.count * item.unit_value,
          unit: item.unit
        }))
      };

      await ingestOrder(orderData);
      setExtractionResult(null);
      loadData();
      setActiveTab('inventory');
    } catch (err) {
      console.error("Ingestion failed", err);
      alert("Failed to ingest order.");
    }
  };

  const handleManualAdd = async () => {
    if (!manualItemName.trim() || !manualItemQty) {
      alert("Please enter item name and quantity");
      return;
    }

    setIsAddingItem(true);
    try {
      await addPantryItem(manualItemName, parseFloat(manualItemQty), manualItemUnit);
      setManualItemName('');
      setManualItemQty('');
      setManualItemUnit('pcs');
      alert("Item added successfully!");
      loadData();
    } catch (err) {
      console.error("Failed to add item", err);
      alert("Failed to add item");
    } finally {
      setIsAddingItem(false);
    }
  };

  const filteredPantry = pantry.filter(item =>
    item.item.name.toLowerCase().includes(searchTerm.toLowerCase())
  );


  const handleSuggestMeal = async () => {
    if (pantry.length === 0) {
      alert('No items in pantry to suggest meals from');
      return;
    }
    setIsLoadingSuggestions(true);
    try {
      const inventory = pantry.map(item => ({
        name: item.item.ingredient?.name || item.item.name,
        quantity: item.effective_quantity,
        unit: item.item.unit
      }));
      const goalsForLLM = goals.map(g => ({
        title: g.title,
        description: g.description || ''
      }));
      // Use selectedMealType instead of getTimeOfDay()
      const result = await suggestMealPersonalized(inventory, goalsForLLM, selectedMealType);
      setMealSuggestions(result.suggestions);
    } catch (err) {
      console.error('Failed to get meal suggestions', err);
      alert('Failed to get meal suggestions: ' + err.message);
    } finally {
      setIsLoadingSuggestions(false);
    }
  };

  const handleAddGoal = async () => {
    if (!newGoalTitle.trim()) {
      alert('Please enter a goal title');
      return;
    }
    try {
      await createGoal(newGoalTitle, newGoalDescription);
      const goalsData = await fetchGoals();
      setGoals(goalsData || []);
      setNewGoalTitle('');
      setNewGoalDescription('');
      setShowAddGoal(false);
    } catch (err) {
      console.error('Failed to create goal', err);
      alert('Failed to create goal');
    }
  };

  const handleDeleteGoal = async (goalId) => {
    try {
      await deleteGoal(goalId);
      setGoals(goals.filter(g => g.id !== goalId));
    } catch (err) {
      console.error('Failed to delete goal', err);
      alert('Failed to delete goal');
    }
  };

  const handleSendChat = async () => {
    if (!chatInput.trim() || isChatLoading) return;

    const userMessage = chatInput.trim();
    setChatInput('');
    setChatMessages(prev => [...prev, { role: 'user', content: userMessage }]);
    setIsChatLoading(true);

    try {
      const inventory = pantry.map(item => ({
        name: item.item.ingredient?.name || item.item.name,
        quantity: item.effective_quantity,
        unit: item.item.unit
      }));
      const goalsForLLM = goals.map(g => ({
        title: g.title,
        description: g.description || ''
      }));

      const result = await sendChatMessage(userMessage, chatMessages, inventory, goalsForLLM);
      setChatMessages(prev => [...prev, { role: 'assistant', content: result.response }]);
    } catch (err) {
      console.error('Chat error', err);
      setChatMessages(prev => [...prev, { role: 'assistant', content: 'Sorry, I encountered an error. Please try again.' }]);
    } finally {
      setIsChatLoading(false);
    }
  };

  const handleNewChat = async () => {
    // Save current conversation if it has messages
    if (chatMessages.length > 0) {
      try {
        await saveConversation(chatMessages);
        // Refresh conversation history
        const convs = await fetchConversations();
        setPastConversations(convs || []);
      } catch (_e) {
        console.error('Failed to save conversation', _e);
      }
    }
    // Clear messages for new chat
    setChatMessages([]);
    setShowConversationHistory(false);
  };

  const loadConversationHistory = async () => {
    try {
      const convs = await fetchConversations();
      setPastConversations(convs || []);
      setShowConversationHistory(true);
    } catch (err) {
      console.error('Failed to load conversations', err);
    }
  };

  // Meal Logging Handlers
  const handleIngredientChange = (e) => {
    const val = e.target.value;
    setIngredientName(val);
    if (val.trim().length > 1) {
      const lower = val.toLowerCase();
      const matches = pantry.filter(p =>
        (p.item.ingredient?.name || p.item.name).toLowerCase().includes(lower)
      ).slice(0, 5);
      setIngredientSuggestions(matches);
    } else {
      setIngredientSuggestions([]);
    }
  };

  const handleSuggestionClick = (pantryItem) => {
    setIngredientName(pantryItem.item.ingredient?.name || pantryItem.item.name);
    setIngredientUnit(pantryItem.item.unit || 'pcs');
    setIngredientQty('1');
    setIngredientSuggestions([]);
  };

  const addIngredient = () => {
    if (!ingredientName.trim()) return;

    const qty = parseFloat(ingredientQty) || 0;
    if (qty <= 0) {
      alert("Please enter a valid quantity");
      return;
    }

    const name = `${qty} ${ingredientUnit} ${ingredientName}`;
    setMealIngredients([...mealIngredients, name]);

    // Reset fields
    setIngredientName('');
    setIngredientQty('1');
    setIngredientUnit('pcs');
    setIngredientSuggestions([]);
  };

  const removeIngredient = (idx) => {
    setMealIngredients(mealIngredients.filter((_, i) => i !== idx));
  };

  const handleLogMealSubmit = async () => {
    if (!mealName.trim() || mealIngredients.length === 0) {
      alert("Please enter a meal name and at least one ingredient");
      return;
    }

    // Prepare meal data for permission check
    // We need to estimate nutrition. The backend does this during log, but for "Can I Eat",
    // we need to send nutrition data.
    // The current frontend doesn't calculate nutrition sum before logging.
    // However, the backend 'checkFoodPermission' requires calories/macros.
    //
    // OPTION 1: We blindly send the ingredients to a new endpoint that calculates AND checks?
    // The spec says "Entry Points: Pantry item, Recent meal...".
    //
    // Let's implement a slight deviation: The "Log Meal" button will first try to VALIDATE.
    // But since we don't have macros on frontend easily (mixed units),
    // we might need to rely on the backend logging response which might return a BLOCK?
    //
    // NO, the spec says "Can I Eat This?" is a separate feature/gate.
    // But the user workflow here is "Log Meal".
    //
    // Let's stick to the Spec: "Pre-check".
    // Implementation:
    // 1. We should probably ask the backend to "Preview" the meal to get macros, OR
    // 2. We just log it and if it's blocked, the backend throws an error with the reason (as per existing logic in `handleLogMealSubmit` catch block).
    //
    // The existing catch block handles 403.
    // Let's see if we can use the `checkFoodPermission` explicitly.
    //
    // For this MVP execution:
    // I will call `checkFoodPermission` if we have nutrition info.
    // But we don't have nutrition info here easily without calculating it.
    //
    // Alternative: The spec implies a specific "Can I Eat This" button/modal.
    // Let's add that separately as requested in the plan: "Add a 'Can I Eat This?' floating action button or menu item".
    //
    // NOTE: The `handleLogMealSubmit` is for the "Log Meal" tab.
    // The spec "Can I Eat This?" is a "Gate".
    //
    // Let's just implement the normal log for now.
    //
    // WAIT, I need to implement the interaction.
    // I added `checkFoodPermission` which takes {calories, protein...}.
    //
    // Let's add a "Check Permission" button in the Log Meal form OR
    // just let the user log and if it fails (because I should enforce it in backend `LogMeal` too?),
    // show the modal.
    //
    // Spec: "It is a gate."
    //
    // Refactoring `handleLogMealSubmit` to use the permission check is complex because we need to resolve ingredients to macros first.
    // The backend `LogMeal` does this.
    //
    // Let's modify `LogMeal` in backend to ALSO return the permission result if it blocks?
    // Current `LogMeal` doesn't seem to block based on `remaining_day_controller.go`?
    // The spec says "Block".
    //
    // I haven't modified `LogMeal` to enforce the block yet.
    // The plan said: "Implement 'Can I eat this?' logic in controller/service".
    // It didn't explicitly say "Modify LogMeal to enforce".
    // It said "Provide users with a **fast, authoritative answer**... It is a **gate**."
    //
    // The "Can I Eat This?" feature is likely a pre-check tool.
    //
    // Let's add a button in the UI "Can I Eat This?" that opens a specific modal to check a food.
    //
    // But for `handleLogMealSubmit` (the main logging),
    // I will leave it as is for now, maybe just calling log.
    //
    // Let's implement the specific "Can I Eat This?" UI check.
    //
    try {
      const response = await logMeal({
        name: mealName,
        ingredients: mealIngredients,
        was_override: requestOverride
      });
      alert("Meal logged successfully!");
      setMealName('');
      setMealIngredients([]);
      setCurrentBlockedReason(null);
      setRequestOverride(false);

      if (response.remaining_state) {
        setRemainingDayState(response.remaining_state);
      } else {
        loadState(); // Fallback
      }

      if (activeTab === 'nutrition') {
        fetchMealHistory().then(setMealHistory);
      }
    } catch (err) {
      // If the backend `LogMeal` were to return 403 with permission data, we could show the modal here.
      // For now, standard alert/error handling.
      if (err.status === 403) {
        setCurrentBlockedReason(err.reason);
      } else {
        console.error("Failed to log meal", err);
        alert("Failed to log meal: " + err.message);
      }
    }
  };

  // Handler for the separate "Can I Eat This?" feature
  const handleCheckFood = async (food) => {
    // food needs to have {name, calories, protein, fat, carbs}
    try {
      const result = await checkFoodPermission(food);
      setCanIEatResult(result);

      // If the result contains food data (from estimation), use it
      const foodName = result.food ? result.food.name : (food.query || food.name);

      // Store pending meal to allow "Log This" from the modal
      setPendingMealLog({
        name: foodName,
        query: food.query,
        ingredients: [`1 serving ${foodName}`],
      });
      setShowCanIEatModal(true);
    } catch (err) {
      console.error("Advice check failed", err);
      alert("Failed to get advice: " + err.message);
      throw err; // Re-throw so the modal knows it failed
    }
  };



  return (
    <div className="App">
      <header className="flex-between mb-8">
        <h1>Pantry.</h1>
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
          {user ? (
            <>
              <div className="badge" style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', background: 'white', border: '1px solid var(--border-color)' }}>
                {user.picture && <img src={user.picture} alt="avatar" style={{ width: 16, height: 16, borderRadius: '50%' }} />}
                <span>{user.name}</span>
              </div>
              <button className="btn btn-secondary" style={{ padding: '0.3rem 0.6rem', fontSize: '0.75rem' }} onClick={() => { setPreferencesForm(userPreferences); setShowPreferencesModal(true); }}>
                ⚙️ Preferences
              </button>
              <button className="btn btn-secondary" style={{ padding: '0.3rem 0.6rem', fontSize: '0.75rem' }} onClick={() => { setUser(null); localStorage.removeItem('token'); }}>
                Logout
              </button>
            </>
          ) : (
            <GoogleLogin
              onSuccess={cred => {
                const decoded = jwtDecode(cred.credential);
                setUser(decoded);
                localStorage.setItem('token', cred.credential);
              }}
              onError={() => console.log('Login Failed')}
            />
          )}
        </div>
      </header>

      <div className="tabs">
        <div className={`tab ${activeTab === 'home' ? 'active' : ''}`} onClick={() => setActiveTab('home')}>Home</div>
        <div className={`tab ${activeTab === 'inventory' ? 'active' : ''}`} onClick={() => setActiveTab('inventory')}>Inventory</div>
        <div className={`tab ${activeTab === 'goals' ? 'active' : ''}`} onClick={() => setActiveTab('goals')}>My Goals</div>
        <div className={`tab ${activeTab === 'nutrition' ? 'active' : ''}`} onClick={() => { setActiveTab('nutrition'); fetchMealHistory().then(setMealHistory).catch(console.error); }}>🔥 Nutrition</div>
        <div className={`tab ${activeTab === 'history' ? 'active' : ''}`} onClick={() => setActiveTab('history')}>Order History</div>
        <div className={`tab ${activeTab === 'log-meal' ? 'active' : ''}`} onClick={() => setActiveTab('log-meal')}>Log Meal</div>
        <div className={`tab ${activeTab === 'upload' ? 'active' : ''}`} onClick={() => setActiveTab('upload')}>Add Items</div>
      </div>

      {activeTab === 'home' && (
        <section>
          <div className="glass-panel p-6 mb-6">
            <h2 className="mb-4">👋 Welcome Back, {user?.name || 'Chef'}!</h2>
            <p className="text-secondary">Here is your daily nutrition overview.</p>
          </div>

          {remainingDayState ? (
            <RemainingDayPanel state={remainingDayState} />
          ) : (
            <div className="glass-panel p-4 mb-6 text-center text-secondary">
              Set daily targets in <strong>My Goals</strong> to see your budget.
            </div>
          )}



          {/* Suggest Meal Button and Type Selection */}
          {user && (
            <div style={{ display: 'flex', justifyContent: 'flex-end', alignItems: 'center', marginBottom: '1rem', gap: '0.5rem' }}>
              <select
                value={selectedMealType}
                onChange={(e) => setSelectedMealType(e.target.value)}
                style={{ padding: '0.5rem', borderRadius: '8px', border: '1px solid var(--border-color)', background: 'var(--glass-bg)', color: 'var(--text-color)' }}
              >
                <option value="Breakfast">Breakfast</option>
                <option value="Lunch">Lunch</option>
                <option value="Dinner">Dinner</option>
                <option value="Snack">Snack</option>
              </select>
              <button
                className="btn"
                onClick={handleSuggestMeal}
                disabled={isLoadingSuggestions}
                style={{ whiteSpace: 'nowrap', background: 'linear-gradient(135deg, #8b5cf6 0%, #6d28d9 100%)', boxShadow: '0 4px 6px -1px rgba(139, 92, 246, 0.4)' }}
              >
                {isLoadingSuggestions ? '✨ Thinking...' : '✨ Suggest a Meal'}
              </button>
              {/* Quick Check Button - Temporary for MVP testing */}
              <button
                className="btn"
                onClick={() => {
                  setCanIEatResult(null); // Reset result to show input form
                  setShowCanIEatModal(true);
                }}
                style={{ whiteSpace: 'nowrap', background: 'linear-gradient(135deg, #10b981 0%, #059669 100%)', marginLeft: '0.5rem' }}
              >
                ❓ Can I Eat This?
              </button>
            </div>
          )}

          {mealSuggestions && (
            <div className="glass-panel p-6 mb-6" style={{ background: 'linear-gradient(135deg, var(--card-bg) 0%, var(--bg-color) 100%)', borderLeft: '4px solid #8b5cf6' }}>
              <div className="flex-between mb-4">
                <h2 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  ✨ Personalized Suggestions
                </h2>
                <button className="icon-btn" onClick={() => setMealSuggestions(null)}>Clear</button>
              </div>

              {(() => {
                try {
                  const data = typeof mealSuggestions === 'string' ? JSON.parse(mealSuggestions) : mealSuggestions;
                  return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                      <div className="text-secondary mb-2">
                        Suggestions for your <strong>{data.meal_type}</strong> based on your goal of <strong>{data.goal}</strong>:
                      </div>
                      <div className="grid-cols-auto">
                        {data.meals.map((meal, i) => (
                          <div key={i} className="glass-panel p-4" style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
                            <h3 style={{ margin: '0 0 0.5rem 0', fontSize: '1rem' }}>{meal.name}</h3>
                            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginBottom: '0.75rem' }}>
                              <span className="badge" style={{ background: '#ede9fe', color: '#6d28d9' }}>🔥 {meal.calories} kcal</span>
                              <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>💪 {meal.protein}g protein</span>
                              {meal.fat && <span className="badge" style={{ background: '#fef3c7', color: '#d97706' }}>🥓 {meal.fat}g fat</span>}
                              {meal.carbs && <span className="badge" style={{ background: '#e0f2fe', color: '#0369a1' }}>🍞 {meal.carbs}g carbs</span>}
                              <span className="badge">⏱️ {meal.prep_time}</span>
                            </div>
                            <div className="text-secondary mb-3" style={{ fontSize: '0.8rem', flex: 1 }}>
                              <strong>Benefits:</strong> {meal.benefits}
                            </div>
                            <button
                              className="btn"
                              style={{ width: '100%', marginTop: 'auto' }}
                              onClick={() => {
                                console.log('View Details clicked, meal:', meal);
                                setSelectedMeal(meal);
                              }}
                            >
                              View Details →
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                } catch {
                  return <pre style={{ whiteSpace: 'pre-wrap', fontSize: '0.875rem' }}>{mealSuggestions}</pre>;
                }
              })()}
            </div>
          )}
        </section>
      )}

      {activeTab === 'log-meal' && (
        <section>
          <div className="glass-panel p-6" style={{ maxWidth: '600px', margin: '0 auto' }}>
            <h2 className="mb-6">Manually Log Meal</h2>

            <div className="mb-4">
              <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: 600 }}>Meal Name</label>
              <input
                type="text"
                placeholder="e.g. Scrambled Eggs with Toast"
                style={{ width: '100%', padding: '0.75rem', borderRadius: '8px', border: '1px solid var(--border-color)' }}
                value={mealName}
                onChange={e => setMealName(e.target.value)}
              />
            </div>

            <div className="mb-4" style={{ position: 'relative' }}>
              <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: 600 }}>Add Ingredients</label>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <input
                  type="number"
                  placeholder="Qty"
                  style={{ width: '80px', padding: '0.75rem', borderRadius: '8px', border: '1px solid var(--border-color)' }}
                  value={ingredientQty}
                  onChange={e => setIngredientQty(e.target.value)}
                />
                <select
                  style={{ width: '90px', padding: '0.75rem', borderRadius: '8px', border: '1px solid var(--border-color)', background: 'white' }}
                  value={ingredientUnit}
                  onChange={e => setIngredientUnit(e.target.value)}
                >
                  <option value="pcs">pcs</option>
                  <option value="g">g</option>
                  <option value="kg">kg</option>
                  <option value="ml">ml</option>
                  <option value="l">l</option>
                  <option value="cup">cup</option>
                  <option value="tbsp">tbsp</option>
                  <option value="tsp">tsp</option>
                  <option value="pack">pack</option>
                  <option value="slice">slice</option>
                </select>
                <input
                  type="text"
                  placeholder="Ingredient Name"
                  style={{ flex: 1, padding: '0.75rem', borderRadius: '8px', border: '1px solid var(--border-color)' }}
                  value={ingredientName}
                  onChange={handleIngredientChange}
                  onKeyPress={e => e.key === 'Enter' && addIngredient()}
                />
                <button className="btn" onClick={addIngredient}>Add</button>
              </div>

              {ingredientSuggestions.length > 0 && (
                <div style={{
                  position: 'absolute',
                  top: '100%',
                  left: 0,
                  right: 0,
                  background: 'white',
                  border: '1px solid var(--border-color)',
                  borderRadius: '8px',
                  marginTop: '0.25rem',
                  zIndex: 10,
                  boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)'
                }}>
                  {ingredientSuggestions.map((item, i) => (
                    <div
                      key={i}
                      style={{ padding: '0.75rem', cursor: 'pointer', borderBottom: i < ingredientSuggestions.length - 1 ? '1px solid #eee' : 'none' }}
                      onClick={() => handleSuggestionClick(item)}
                      onMouseOver={e => e.currentTarget.style.background = '#f9fafb'}
                      onMouseOut={e => e.currentTarget.style.background = 'white'}
                    >
                      <div style={{ fontWeight: 500 }}>{item.item.ingredient?.name || item.item.name}</div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                        In Stock: {item.effective_quantity} {item.item.unit}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="mb-6">
              <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: 600 }}>Ingredients List</label>
              {mealIngredients.length === 0 ? (
                <div className="text-secondary" style={{ fontStyle: 'italic', padding: '1rem', background: 'rgba(0,0,0,0.02)', borderRadius: '8px', textAlign: 'center' }}>
                  No ingredients added yet.
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                  {mealIngredients.map((ing, idx) => (
                    <div key={idx} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0.75rem', background: 'white', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                      <span>{ing}</span>
                      <button
                        style={{ background: 'none', border: 'none', color: 'var(--danger)', cursor: 'pointer', fontSize: '1.2rem' }}
                        onClick={() => removeIngredient(idx)}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <MealBlockedBanner
              reason={currentBlockedReason}
              onOverride={() => {
                setRequestOverride(true);
                // Optionally auto-submit
                // handleLogMealSubmit(); 
                // Better to let user click "Log Meal" again which now has override flag set?
                // But the banner says "I understand, log anyway".
                // If they click that, we should probably submit.
                // But requestOverride state might not update immediately for next line execution.
                // So we'll set state, and maybe call a function that passes true.
                // Or simple: setRequestOverride(true) and change "Log Meal" button text?
                // Let's make the banner button trigger the log with override.

                // Actually, simpler: Set RequestOverride(true) and let user click Log Meal again.
                // The banner persists until we clear it.
              }}
            />
            {/* If override is requested, maybe highlight the button */}

            <button
              className="btn w-full"
              style={{
                padding: '1rem',
                fontSize: '1rem',
                background: requestOverride ? 'linear-gradient(135deg, #ef4444 0%, #b91c1c 100%)' : undefined
              }}
              onClick={handleLogMealSubmit}
            >
              {requestOverride ? 'Confirm Log (Override)' : 'Log Meal'}
            </button>
          </div>
        </section>
      )}


      {activeTab === 'inventory' && (
        <section>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '1rem', marginBottom: '1rem' }}>

            <div className="search-container" style={{ marginBottom: 0 }}>
              <span style={{ position: 'absolute', left: '0.75rem', top: '0.55rem', color: 'var(--text-secondary)', fontSize: '0.8rem' }}>🔍</span>
              <input
                className="search-input"
                placeholder="Search pantry..."
                value={searchTerm}
                onChange={e => setSearchTerm(e.target.value)}
              />
            </div>

          </div>



          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem' }}><div className="loader"></div></div>
          ) : !user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to view your pantry.</div>
          ) : filteredPantry.length === 0 ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>No items found.</div>
          ) : (
            <>
              {selectedIds.size > 0 && (
                <div className="glass-panel p-4 mb-4 flex-between" style={{ background: 'var(--danger-light)', borderColor: 'var(--danger)', borderLeftWidth: '4px' }}>
                  <span style={{ fontWeight: 600 }}>{selectedIds.size} items selected</span>
                  <button className="btn" style={{ background: 'var(--danger)' }} onClick={handleBulkDelete}>Delete Selected</button>
                </div>
              )}
              <div className="glass-panel p-6">
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr style={{ borderBottom: '2px solid var(--border-color)', textAlign: 'left' }}>
                      <th style={{ padding: '0.75rem 0', width: '40px' }}>
                        <input
                          type="checkbox"
                          checked={selectedIds.size === filteredPantry.length && filteredPantry.length > 0}
                          onChange={handleSelectAll}
                        />
                      </th>
                      <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Item</th>
                      <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Stock</th>
                      <th style={{ padding: '0.75rem 0', fontWeight: 600 }}>Unit</th>
                      <th style={{ padding: '0.75rem 0', fontWeight: 600, textAlign: 'right' }}>Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredPantry.map(item => {
                      const isLow = item.effective_quantity < 2;
                      const isSelected = selectedIds.has(item.item_id);
                      return (
                        <tr key={item.id} style={{ borderBottom: '1px solid var(--border-color)', background: isSelected ? 'rgba(0,0,0,0.02)' : 'transparent' }}>
                          <td style={{ padding: '0.75rem 0' }}>
                            <input
                              type="checkbox"
                              checked={isSelected}
                              onChange={() => handleSelectItem(item.item_id)}
                            />
                          </td>
                          <td style={{ padding: '0.75rem 0' }}>
                            <div style={{ fontWeight: 600 }}>{item.ingredient?.name || item.item.name}</div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'flex', alignItems: 'center' }}>
                              {item.item.brand?.name && (
                                <span className="badge" style={{ padding: '0.1rem 0.3rem', marginRight: '0.4rem', background: '#fef3c7', color: '#92400e', fontSize: '0.65rem' }}>
                                  {item.item.brand.name}
                                </span>
                              )}
                              {item.item.product_name || item.item.name}
                            </div>
                            {item.item.calories > 0 && (
                              <div style={{ fontSize: '0.65rem', color: 'var(--text-secondary)', marginTop: '0.2rem' }}>
                                <strong>🔥 {Math.round(item.item.calories)}</strong> {
                                  ['pc', 'pcs', 'unit', 'units', 'piece', 'pieces', 'pack', 'dozen'].includes(item.item.unit?.toLowerCase())
                                    ? `kcal/${item.item.unit.toLowerCase()}`
                                    : `kcal/100${item.item.unit === 'ml' ? 'ml' : 'g'}`
                                } | P: {item.item.protein?.toFixed(1)}g | C: {item.item.carbs?.toFixed(1)}g | F: {item.item.fat?.toFixed(1)}g
                                {!item.item.nutrition_verified && <span style={{ fontStyle: 'italic', marginLeft: '0.4rem', opacity: 0.7 }}>(Est.)</span>}
                              </div>
                            )}
                          </td>
                          <td style={{ padding: '0.75rem 0', fontWeight: 600, color: isLow ? 'var(--danger)' : 'inherit' }}>
                            {item.effective_quantity}
                          </td>
                          <td style={{ padding: '0.75rem 0' }}><span className="badge">{item.item.unit}</span></td>
                          <td style={{ padding: '0.75rem 0', textAlign: 'right' }}>
                            <button className="icon-btn" style={{ marginRight: '0.5rem', color: 'var(--accent-color)' }} onClick={() => setViewingNutritionItem(item.item)}>📊</button>
                            <button className="icon-btn" onClick={() => handleEdit(item)}>Update</button>
                            <button className="icon-btn" style={{ marginLeft: '0.5rem', color: 'var(--danger)' }} onClick={() => handleDelete(item)}>Delete</button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </section>
      )}

      {activeTab === 'goals' && (
        <section>
          {!user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to manage your goals.</div>
          ) : loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem' }}><div className="loader"></div></div>
          ) : (
            <>
              <div className="glass-panel p-6 mb-4">
                <div className="flex-between mb-4">
                  <h2 style={{ margin: 0 }}>🎯 My Health Goals</h2>
                  <button className="btn" onClick={() => setShowAddGoal(true)}>+ Add Goal</button>
                </div>
                {goals.length === 0 ? (
                  <p className="text-secondary">No goals set yet. Add a goal to get personalized meal suggestions!</p>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                    {goals.map(goal => (
                      <div key={goal.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', padding: '1rem', background: 'rgba(0,0,0,0.02)', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                        <div>
                          <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>{goal.title}</div>
                          {goal.description && <div className="text-secondary" style={{ fontSize: '0.875rem' }}>{goal.description}</div>}
                        </div>

                        <div style={{ display: 'flex', gap: '0.5rem' }}>
                          <button
                            className="btn btn-secondary"
                            style={{ fontSize: '0.8rem', padding: '0.3rem 0.6rem' }}
                            onClick={() => {
                              setTargetModalGoalId(goal.id);
                              // Fetch current targets or use defaults?
                              // The goal object might not have targets yet if the API doesn't return them in list.
                              // We might need to fetch them or just open modal with defaults/previous.
                              // For now, let's assume we start fresh or pass what we have.
                              // Ideally fetchRemainingDayState gave us profile.
                              // But that profile is for the *active* goal or user?
                              // The backend links profile to goal.
                              // Let's just open modal and let it handle defaults if not passed.
                              setShowTargetModal(true);
                            }}
                          >
                            🎯 Set Targets
                          </button>
                          <button
                            onClick={() => handleDeleteGoal(goal.id)}
                            style={{ background: 'transparent', border: 'none', color: 'var(--danger)', cursor: 'pointer', fontSize: '1.2rem' }}
                          >×</button>
                        </div>

                      </div>
                    ))}
                  </div>
                )}
              </div>
              <div className="glass-panel p-6" style={{ background: 'linear-gradient(135deg, #f5f7fa 0%, #e4e8eb 100%)' }}>
                <h3 style={{ margin: '0 0 0.5rem 0' }}>💡 Goal Ideas</h3>
                <p className="text-secondary mb-4" style={{ fontSize: '0.875rem' }}>Click to add any of these common goals:</p>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                  {[
                    { title: 'Lose 5kg in 30 days', desc: 'Focus on low-calorie, high-protein meals' },
                    { title: 'Build muscle', desc: 'High protein diet with complex carbs' },
                    { title: 'Healthy pregnancy diet', desc: 'Nutrient-rich meals for expecting mothers' },
                    { title: 'Train for 10K run', desc: 'Endurance-focused nutrition plan' },
                    { title: 'Reduce sugar intake', desc: 'Cut down on processed sugars' },
                    { title: 'Heart-healthy eating', desc: 'Low sodium, good fats' },
                  ].map((preset, i) => (
                    <button
                      key={i}
                      className="btn btn-secondary"
                      style={{ fontSize: '0.8rem', padding: '0.4rem 0.8rem' }}
                      onClick={async () => {
                        try {
                          await createGoal(preset.title, preset.desc);
                          const goalsData = await fetchGoals();
                          setGoals(goalsData || []);
                        } catch {
                          alert('Failed to add goal');
                        }
                      }}
                    >
                      {preset.title}
                    </button>
                  ))}
                </div>
              </div>
            </>
          )}
        </section>
      )}


      {activeTab === 'nutrition' && (
        <section>


          {!user ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to view nutrition insights.</div>
          ) : mealHistory.length === 0 ? (
            <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>No meals logged yet to analyze. Log a meal from the suggested recipes!</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
              {(() => {
                // Group meals by date
                const grouped = {};
                mealHistory.forEach(meal => {
                  const dateInfo = new Date(meal.logged_at);
                  const dateKey = dateInfo.toLocaleDateString('en-US', { weekday: 'long', month: 'short', day: 'numeric' });

                  if (!grouped[dateKey]) {
                    grouped[dateKey] = {
                      date: dateInfo,
                      meals: [],
                      totals: { calories: 0, protein: 0, carbs: 0, fat: 0 }
                    };
                  }

                  grouped[dateKey].meals.push(meal);
                  grouped[dateKey].totals.calories += meal.calories || 0;
                  grouped[dateKey].totals.protein += meal.protein || 0;
                  grouped[dateKey].totals.carbs += meal.carbs || 0;
                  grouped[dateKey].totals.fat += meal.fat || 0;
                });

                // Sort dates descending
                const sortedDates = Object.keys(grouped).sort((a, b) => grouped[b].date - grouped[a].date);

                return sortedDates.map(dateKey => {
                  const dayData = grouped[dateKey];
                  return (
                    <div key={dateKey}>
                      <div className="flex-between mb-2">
                        <h3 style={{ margin: 0, color: 'var(--text-primary)' }}>{dateKey}</h3>
                        <div className="text-secondary" style={{ fontSize: '0.9rem' }}>
                          Total: <strong style={{ color: 'var(--accent-color)' }}>{dayData.totals.calories.toFixed(0)} kcal</strong>
                        </div>
                      </div>

                      {/* Daily Macro Summary Bar */}
                      <div className="glass-panel p-4 mb-4" style={{ background: '#f8fafc', border: 'none' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-around', textAlign: 'center' }}>
                          <div>
                            <div style={{ fontSize: '1.2rem', fontWeight: 600, color: '#6d28d9' }}>{dayData.totals.protein.toFixed(1)}g</div>
                            <div className="text-secondary" style={{ fontSize: '0.75rem' }}>Protein</div>
                          </div>
                          <div>
                            <div style={{ fontSize: '1.2rem', fontWeight: 600, color: '#dc2626' }}>{dayData.totals.carbs.toFixed(1)}g</div>
                            <div className="text-secondary" style={{ fontSize: '0.75rem' }}>Carbs</div>
                          </div>
                          <div>
                            <div style={{ fontSize: '1.2rem', fontWeight: 600, color: '#d97706' }}>{dayData.totals.fat.toFixed(1)}g</div>
                            <div className="text-secondary" style={{ fontSize: '0.75rem' }}>Fat</div>
                          </div>
                        </div>
                      </div>

                      <div className="grid-cols-auto">
                        {dayData.meals.map(meal => (
                          <div
                            key={meal.id}
                            className="glass-panel p-4"
                            style={{ position: 'relative', cursor: 'pointer', transition: 'transform 0.2s' }}
                            onClick={() => setSelectedMeal(meal)}
                          >
                            <button
                              className="icon-btn"
                              style={{ position: 'absolute', top: '0.5rem', right: '0.5rem', color: 'var(--danger)', zIndex: 10 }}
                              onClick={async (e) => {
                                e.stopPropagation(); // Prevent opening modal
                                if (confirm('Delete this meal and restore ingredients to pantry?')) {
                                  try {
                                    await deleteMealLog(meal.id);
                                    setMealHistory(mealHistory.filter(m => m.id !== meal.id));
                                    const data = await fetchPantry();
                                    setPantry(data);
                                    alert('Meal deleted and pantry restored!');
                                  } catch (err) {
                                    console.error('Failed to delete meal', err);
                                    alert('Failed to delete meal');
                                  }
                                }
                              }}
                            >
                              🗑️
                            </button>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>
                              {new Date(meal.logged_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                            </div>
                            <h4 style={{ margin: '0 0 0.5rem 0', fontSize: '1rem', paddingRight: '1.5rem' }}>{meal.name}</h4>
                            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                              <span className="badge" style={{ background: '#ede9fe', color: '#6d28d9' }}>🔥 {meal.calories?.toFixed(0)}</span>
                              <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>💪 {meal.protein?.toFixed(1)}g</span>
                            </div>
                          </div>
                        ))}
                      </div>
                      <div style={{ height: '1px', background: 'var(--border-color)', margin: '2rem 0' }}></div>
                    </div>
                  );
                });
              })()}
            </div>
          )}
        </section>
      )}

      {
        activeTab === 'history' && (
          <section>
            {!user ? (
              <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>Please log in to view order history.</div>
            ) : loading ? (
              <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem' }}><div className="loader"></div></div>
            ) : orders.length === 0 ? (
              <div className="glass-panel p-6 text-secondary" style={{ textAlign: 'center' }}>No orders found.</div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                {orders.map(order => (
                  <div key={order.id} className="glass-panel p-6">
                    <div className="flex-between mb-4">
                      <div>
                        <div style={{ fontWeight: 600 }}>{order.provider.toUpperCase()} #{order.external_order_id.slice(-6)}</div>
                        <div className="text-secondary">{new Date(order.order_date).toLocaleDateString()}</div>
                      </div>
                      <span className="badge">{order.items?.length || 0} Items</span>
                    </div>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                      {order.items?.map((item, i) => (
                        <div key={i} className="badge" style={{ background: 'transparent', border: '1px solid var(--border-color)', display: 'flex', flexDirection: 'column', alignItems: 'flex-start', padding: '0.5rem 0.75rem', borderRadius: '8px' }}>
                          <div style={{ fontWeight: 600, fontSize: '0.85rem' }}>{item.item?.ingredient?.name || item.item?.name || item.raw_name}</div>
                          {(item.item?.brand?.name || item.item?.product_name) && (
                            <div style={{ fontSize: '0.7rem', color: 'var(--text-secondary)' }}>
                              {item.item?.brand?.name} {item.item?.product_name}
                            </div>
                          )}
                          <div style={{ fontSize: '0.75rem', marginTop: '0.25rem', fontWeight: 500 }}>Qty: {item.quantity}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        )
      }

      {
        activeTab === 'upload' && (
          <section>
            <div className="glass-panel p-6 mb-6">
              <h2 className="mb-4">Manual Entry</h2>
              <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-end' }}>
                <div style={{ flex: 2 }}>
                  <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.875rem' }}>Item Name</label>
                  <input
                    type="text"
                    placeholder="e.g. Milk, Eggs"
                    style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px' }}
                    value={manualItemName}
                    onChange={e => setManualItemName(e.target.value)}
                  />
                </div>
                <div style={{ flex: 1 }}>
                  <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.875rem' }}>Quantity</label>
                  <input
                    type="number"
                    placeholder="0"
                    style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px' }}
                    value={manualItemQty}
                    onChange={e => setManualItemQty(e.target.value)}
                  />
                </div>
                <div style={{ flex: 1 }}>
                  <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.875rem' }}>Unit</label>
                  <select
                    style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px', background: 'white' }}
                    value={manualItemUnit}
                    onChange={e => setManualItemUnit(e.target.value)}
                  >
                    <option value="pcs">pcs</option>
                    <option value="kg">kg</option>
                    <option value="g">g</option>
                    <option value="ml">ml</option>
                    <option value="l">l</option>
                    <option value="pack">pack</option>
                  </select>
                </div>
                <button
                  className="btn"
                  onClick={handleManualAdd}
                  disabled={isAddingItem}
                  style={{ height: '38px', minWidth: '100px' }}
                >
                  {isAddingItem ? 'Adding...' : 'Add Item'}
                </button>
              </div>
            </div>

            {!extractionResult ? (
              <div className="glass-panel p-6" style={{ textAlign: 'center', border: '2px dashed var(--border-color)', background: 'transparent' }}>
                <h2 className="mb-2">Image Upload</h2>
                <p className="text-secondary mb-6">Upload image to update inventory automatically.</p>
                <label className="btn" style={{ cursor: 'pointer' }}>
                  {isExtracting ? <div className="loader" style={{ width: 14, height: 14 }}></div> : "Select File"}
                  <input type="file" hidden onChange={handleFileUpload} accept=".pdf,.jpg,.png" />
                </label>
              </div>
            ) : (
              <div className="glass-panel p-6">
                <div className="flex-between mb-6">
                  <h2 style={{ margin: 0 }}>Review {extractionResult.provider} Items</h2>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <button className="btn btn-secondary" onClick={() => setExtractionResult(null)}>Cancel</button>
                    <button className="btn" onClick={handleConfirmIngest}>Confirm</button>
                  </div>
                </div>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border-color)', textAlign: 'left', color: 'var(--text-secondary)' }}>
                      <th style={{ padding: '0.5rem 0' }}>Name</th>
                      <th style={{ padding: '0.5rem 0' }}>Qty</th>
                      <th style={{ padding: '0.5rem 0' }}>Size</th>
                      <th style={{ padding: '0.5rem 0' }}>Unit</th>
                      <th style={{ padding: '0.5rem 0' }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {extractionResult.items.map((item, idx) => (
                      <tr key={idx} style={{ borderBottom: '1px solid var(--border-color)' }}>
                        <td style={{ padding: '0.5rem 0' }}>
                          {/* {console.log(extractionResult)} */}
                          <input style={{ background: 'transparent', border: 'none', width: '100%' }} value={item.name} onChange={e => { const n = { ...extractionResult }; n.items[idx].name = e.target.value; setExtractionResult(n); }} />
                        </td>
                        <td style={{ padding: '0.5rem 0' }}><input type="number" style={{ background: 'transparent', border: 'none', width: '40px' }} value={item.count} onChange={e => { const n = { ...extractionResult }; n.items[idx].count = parseFloat(e.target.value); setExtractionResult(n); }} /></td>
                        <td style={{ padding: '0.5rem 0' }}><input type="number" style={{ background: 'transparent', border: 'none', width: '40px' }} value={item.unit_value} onChange={e => { const n = { ...extractionResult }; n.items[idx].unit_value = parseFloat(e.target.value); setExtractionResult(n); }} /></td>
                        <td style={{ padding: '0.5rem 0' }}><input style={{ background: 'transparent', border: 'none', width: '40px' }} value={item.unit} onChange={e => { const n = { ...extractionResult }; n.items[idx].unit = e.target.value; setExtractionResult(n); }} /></td>
                        <td style={{ textAlign: 'right' }}><button style={{ background: 'transparent', border: 'none', color: 'var(--danger)', cursor: 'pointer' }} onClick={() => { const n = { ...extractionResult }; n.items = n.items.filter((_, i) => i !== idx); setExtractionResult(n); }}>✕</button></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        )
      }

      {
        editingId && (
          <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
            <div className="glass-panel p-6" style={{ width: '280px' }}>
              <h2 className="mb-4">Update Stock</h2>
              <input
                type="number"
                className="mb-4"
                style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px' }}
                value={editValue}
                onChange={e => setEditValue(e.target.value)}
                autoFocus
              />
              <div className="flex-between">
                <button className="btn btn-secondary" onClick={() => setEditingId(null)}>Cancel</button>
                <button className="btn" onClick={() => handleSave(editingId)}>Update</button>
              </div>
            </div>
          </div>
        )
      }

      {
        deletingItem && (
          <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
            <div className="glass-panel p-6" style={{ width: '320px' }}>
              <h2 className="mb-4">Delete Item</h2>
              <p className="text-secondary mb-6">Are you sure you want to delete <strong>{deletingItem.item.name}</strong> from your pantry?</p>
              <div className="flex-between">
                <button className="btn btn-secondary" onClick={() => setDeletingItem(null)}>Cancel</button>
                <button className="btn" style={{ background: 'var(--danger)' }} onClick={handleConfirmDelete}>Delete</button>
              </div>
            </div>
          </div>
        )
      }

      {
        showBulkDeleteConfirm && (
          <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
            <div className="glass-panel p-6" style={{ width: '320px' }}>
              <h2 className="mb-4">Delete Multiple Items</h2>
              <p className="text-secondary mb-6">Are you sure you want to delete <strong>{selectedIds.size}</strong> selected items from your pantry?</p>
              <div className="flex-between">
                <button className="btn btn-secondary" onClick={() => setShowBulkDeleteConfirm(false)}>Cancel</button>
                <button className="btn" style={{ background: 'var(--danger)' }} onClick={handleConfirmBulkDelete}>Delete All</button>
              </div>
            </div>
          </div>
        )
      }

      {
        showAddGoal && (
          <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
            <div className="glass-panel p-6" style={{ width: '400px' }}>
              <h2 className="mb-4">🎯 Add Health Goal</h2>
              <div className="mb-4">
                <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 500, marginBottom: '0.25rem' }}>Goal Title</label>
                <input
                  type="text"
                  placeholder="e.g. Lose 5kg, Build Muscle"
                  style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px' }}
                  value={newGoalTitle}
                  onChange={e => setNewGoalTitle(e.target.value)}
                  autoFocus
                />
              </div>
              <div className="mb-6">
                <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 500, marginBottom: '0.25rem' }}>Description (optional)</label>
                <textarea
                  placeholder="Give more context to the AI..."
                  style={{ width: '100%', padding: '0.5rem', border: '1px solid var(--border-color)', borderRadius: '8px', minHeight: '80px', fontFamily: 'inherit' }}
                  value={newGoalDescription}
                  onChange={e => setNewGoalDescription(e.target.value)}
                />
              </div>
              <div className="flex-between">
                <button className="btn btn-secondary" onClick={() => setShowAddGoal(false)}>Cancel</button>
                <button className="btn" onClick={handleAddGoal}>Save Goal</button>
              </div>
            </div>
          </div>
        )
      }
      {/* Nutrition Detail Modal */}
      {
        viewingNutritionItem && (
          <div className="modal-overlay" onClick={() => setViewingNutritionItem(null)}>
            <div className="glass-panel p-8" onClick={e => e.stopPropagation()} style={{ maxWidth: '400px', width: '90%', position: 'relative' }}>
              <button className="icon-btn" onClick={() => setViewingNutritionItem(null)} style={{ position: 'absolute', right: '1rem', top: '1rem', fontSize: '1.25rem' }}>×</button>
              <div className="mb-6">
                <h2 style={{ margin: '0 0 0.25rem 0' }}>{viewingNutritionItem.ingredient?.name || viewingNutritionItem.name}</h2>
                <div className="text-secondary" style={{ fontSize: '0.9rem' }}>
                  {viewingNutritionItem.brand?.name && <span className="badge" style={{ padding: '0.1rem 0.3rem', marginRight: '0.5rem' }}>{viewingNutritionItem.brand.name}</span>}
                  {viewingNutritionItem.product_name}
                </div>
              </div>

              <div style={{ background: 'rgba(0,0,0,0.02)', borderRadius: '12px', padding: '1.5rem', marginBottom: '1.5rem' }}>
                <div style={{ textAlign: 'center', marginBottom: '1.5rem' }}>
                  <div style={{ fontSize: '2.5rem', fontWeight: 700, lineHeight: 1 }}>{Math.round(viewingNutritionItem.calories)}</div>
                  <div className="text-secondary" style={{ fontSize: '0.8rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                    Calories / {['pc', 'pcs', 'unit', 'units', 'piece', 'pieces', 'pack', 'dozen'].includes(viewingNutritionItem.unit?.toLowerCase())
                      ? viewingNutritionItem.unit.toLowerCase()
                      : `100${viewingNutritionItem.unit === 'ml' ? 'ml' : 'g'}`}
                  </div>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                  {[
                    { label: 'Protein', value: viewingNutritionItem.protein, color: '#8b5cf6', suffix: 'g' },
                    { label: 'Carbs', value: viewingNutritionItem.carbs, color: '#3b82f6', suffix: 'g' },
                    { label: 'Fats', value: viewingNutritionItem.fat, color: '#f59e0b', suffix: 'g' },
                    { label: 'Fiber', value: viewingNutritionItem.fiber, color: '#10b981', suffix: 'g' },
                  ].map((macro, i) => (
                    <div key={i} style={{ padding: '0.75rem', background: 'white', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                      <div className="text-secondary" style={{ fontSize: '0.7rem', marginBottom: '0.2rem' }}>{macro.label}</div>
                      <div style={{ fontWeight: 600, fontSize: '1.1rem', color: macro.color }}>{macro.value?.toFixed(1)}{macro.suffix}</div>
                    </div>
                  ))}
                </div>
              </div>

              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', textAlign: 'center' }}>
                {viewingNutritionItem.nutrition_verified ? (
                  <span style={{ color: '#10b981' }}>● Verified Data</span>
                ) : (
                  <span style={{ fontStyle: 'italic' }}>● Estimated by AI based on ingredient type</span>
                )}
              </div>

              <button className="btn btn-secondary w-full mt-6" onClick={() => setViewingNutritionItem(null)}>Close</button>
            </div>
          </div>
        )
      }

      {/* Meal Detail Modal */}
      {
        selectedMeal && (
          <div className="modal-overlay" onClick={() => setSelectedMeal(null)}>
            <div className="modal glass-panel p-6" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '500px' }}>
              <div className="flex-between mb-4">
                <h2 style={{ margin: 0 }}>{selectedMeal.name}</h2>
                <button className="icon-btn" onClick={() => setSelectedMeal(null)}>✕</button>
              </div>

              <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
                <span className="badge" style={{ background: '#ede9fe', color: '#6d28d9' }}>🔥 {typeof selectedMeal.calories === 'number' ? selectedMeal.calories.toFixed(0) : selectedMeal.calories} kcal</span>
                <span className="badge" style={{ background: '#dcfce7', color: '#15803d' }}>💪 {typeof selectedMeal.protein === 'number' ? selectedMeal.protein.toFixed(1) : selectedMeal.protein}g protein</span>
                {selectedMeal.fat && <span className="badge" style={{ background: '#fef3c7', color: '#d97706' }}>🥓 {typeof selectedMeal.fat === 'number' ? selectedMeal.fat.toFixed(1) : selectedMeal.fat}g fat</span>}
                {selectedMeal.carbs && <span className="badge" style={{ background: '#e0f2fe', color: '#0369a1' }}>🍞 {typeof selectedMeal.carbs === 'number' ? selectedMeal.carbs.toFixed(1) : selectedMeal.carbs}g carbs</span>}
                {selectedMeal.prep_time && <span className="badge">⏱️ {selectedMeal.prep_time}</span>}
              </div>

              {selectedMeal.benefits && (
                <div style={{ marginBottom: '1rem' }}>
                  <div style={{ fontWeight: 600, marginBottom: '0.5rem', color: 'var(--accent-color)' }}>Benefits</div>
                  <div className="text-secondary">{selectedMeal.benefits}</div>
                </div>
              )}

              <div style={{ marginBottom: '1rem' }}>
                <div style={{ fontWeight: 600, marginBottom: '0.5rem', color: 'var(--accent-color)' }}>Ingredients</div>
                <ul style={{ margin: 0, paddingLeft: '1.2rem' }}>
                  {(() => {
                    const ings = typeof selectedMeal.ingredients === 'string'
                      ? JSON.parse(selectedMeal.ingredients || '[]')
                      : selectedMeal.ingredients || [];
                    return ings.map((ing, i) => (
                      <li key={i} className="text-secondary" style={{ marginBottom: '0.25rem' }}>{ing}</li>
                    ));
                  })()}
                </ul>
              </div>

              {selectedMeal.instructions && (
                <div style={{ marginBottom: '1.5rem' }}>
                  <div style={{ fontWeight: 600, marginBottom: '0.5rem', color: 'var(--accent-color)' }}>Instructions</div>
                  <div className="text-secondary">{selectedMeal.instructions}</div>
                </div>
              )}

              {/* Allow re-logging historical meals */}
              <button
                className="btn w-full"
                style={{ background: 'linear-gradient(135deg, #10b981 0%, #059669 100%)', padding: '1rem' }}
                disabled={isLoggingMeal}
                onClick={async () => {
                  setIsLoggingMeal(true);
                  try {
                    // Create a clean meal object to log (remove ID if it's history, so new log is created)
                    // The backend creates a new log anyway based on passed data.
                    const mealToLog = { ...selectedMeal };
                    if (mealToLog.logged_at) {
                      delete mealToLog.id; // Ensure new ID is generated
                      delete mealToLog.logged_at;
                    }

                    if (typeof mealToLog.ingredients === 'string') {
                      try {
                        mealToLog.ingredients = JSON.parse(mealToLog.ingredients);
                      } catch {
                        mealToLog.ingredients = [];
                      }
                    }

                    const result = await logMeal(mealToLog);
                    alert(`✅ Meal logged! Updated: ${result.updated_items?.join(', ') || 'No pantry items matched'}`);
                    setSelectedMeal(null);

                    if (result.remaining_state) {
                      setRemainingDayState(result.remaining_state);
                      if (result.next_action) {
                        setNextAction(result.next_action);
                      } else {
                        loadState();
                      }
                    } else {
                      loadState();
                    }

                    // Refresh pantry to show updated quantities
                    const data = await fetchPantry();
                    setPantry(data);
                    // Refresh history
                    if (activeTab === 'nutrition') {
                      const history = await fetchMealHistory();
                      setMealHistory(history);
                    }
                  } catch (err) {
                    console.error('Failed to log meal', err);
                    alert('Failed to log meal: ' + err.message);
                  } finally {
                    setIsLoggingMeal(false);
                  }
                }}
              >
                {isLoggingMeal ? '⏳ Logging...' : (selectedMeal.logged_at ? '📋 Log As New Meal' : '📋 Log This Meal')}
              </button>

              {selectedMeal.logged_at && (
                <div className="text-secondary" style={{ fontSize: '0.8rem', textAlign: 'center', marginTop: '1rem', fontStyle: 'italic' }}>
                  Originally logged on {new Date(selectedMeal.logged_at).toLocaleString()}
                </div>
              )}
            </div>
          </div>
        )
      }


      {/* Target Modal */}
      {showTargetModal && (
        <MacroTargetModal
          goalId={targetModalGoalId}
          currentTargets={null} // We could pass existing if available
          onClose={() => setShowTargetModal(false)}
          onSave={async (gid, targets) => {
            try {
              await setGoalMacroTargets(gid, targets);
              setShowTargetModal(false);
              alert("Targets updated!");
              loadState(); // Refresh state
            } catch {
              alert("Failed to set targets");
            }
          }}
        />
      )}

      {/* Floating Chatbot */}

      {user && (
        <>
          {/* Chat Toggle Button */}
          <button
            onClick={() => setChatOpen(!chatOpen)}
            style={{
              position: 'fixed',
              bottom: '24px',
              right: '24px',
              width: '60px',
              height: '60px',
              borderRadius: '50%',
              background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
              border: 'none',
              boxShadow: '0 4px 15px rgba(102, 126, 234, 0.4)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '1.5rem',
              color: 'white',
              zIndex: 1000,
              transition: 'transform 0.2s'
            }}
            onMouseOver={(e) => e.currentTarget.style.transform = 'scale(1.1)'}
            onMouseOut={(e) => e.currentTarget.style.transform = 'scale(1)'}
          >
            {chatOpen ? '✕' : '💬'}
          </button>

          {/* Chat Window */}
          {chatOpen && (
            <div style={{
              position: 'fixed',
              bottom: '100px',
              right: '24px',
              width: '380px',
              height: '500px',
              background: 'white',
              borderRadius: '16px',
              boxShadow: '0 10px 40px rgba(0,0,0,0.2)',
              display: 'flex',
              flexDirection: 'column',
              zIndex: 999,
              overflow: 'hidden'
            }}>
              {/* Chat Header */}
              <div style={{
                background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                color: 'white',
                padding: '0.75rem 1rem',
                fontWeight: 600,
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center'
              }}>
                <div>
                  🤖 Kitchen Assistant
                  <div style={{ fontSize: '0.7rem', opacity: 0.9, fontWeight: 400 }}>Ask about your pantry, meals & nutrition</div>
                </div>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <button
                    onClick={loadConversationHistory}
                    style={{ background: 'rgba(255,255,255,0.2)', border: 'none', borderRadius: '6px', padding: '0.4rem 0.6rem', color: 'white', cursor: 'pointer', fontSize: '0.75rem' }}
                    title="Past conversations"
                  >
                    📜
                  </button>
                  <button
                    onClick={handleNewChat}
                    style={{ background: 'rgba(255,255,255,0.2)', border: 'none', borderRadius: '6px', padding: '0.4rem 0.6rem', color: 'white', cursor: 'pointer', fontSize: '0.75rem' }}
                    title="New conversation"
                  >
                    ✨ New
                  </button>
                </div>
              </div>

              {/* Messages or History */}
              <div style={{
                flex: 1,
                overflowY: 'auto',
                padding: '1rem',
                display: 'flex',
                flexDirection: 'column',
                gap: '0.75rem'
              }}>
                {showConversationHistory ? (
                  <>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
                      <span style={{ fontWeight: 600, color: '#333' }}>📜 Past Conversations</span>
                      <button onClick={() => setShowConversationHistory(false)} style={{ background: 'none', border: 'none', color: '#667eea', cursor: 'pointer', fontSize: '0.85rem' }}>← Back</button>
                    </div>
                    {pastConversations.length === 0 ? (
                      <div style={{ color: '#999', textAlign: 'center', marginTop: '2rem' }}>No saved conversations yet</div>
                    ) : (
                      pastConversations.map((conv, idx) => (
                        <div key={idx} style={{ padding: '0.75rem', background: '#f8f9fa', borderRadius: '8px', borderLeft: '3px solid #667eea' }}>
                          <div style={{ fontSize: '0.8rem', color: '#999', marginBottom: '0.25rem' }}>{conv.created_at}</div>
                          <div style={{ fontSize: '0.9rem', color: '#333' }}>{conv.summary}</div>
                        </div>
                      ))
                    )}
                  </>
                ) : (
                  <>
                    {chatMessages.length === 0 && (
                      <div style={{ color: 'var(--text-secondary)', textAlign: 'center', marginTop: '2rem' }}>
                        <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>👋</div>
                        <div>Hi! Ask me anything about your pantry or meals.</div>
                        <div style={{ fontSize: '0.8rem', marginTop: '1rem', color: '#999' }}>
                          Try: "What can I make for dinner?" or "What's running low?"
                        </div>
                      </div>
                    )}
                    {chatMessages.map((msg, idx) => (
                      <div key={idx} style={{
                        alignSelf: msg.role === 'user' ? 'flex-end' : 'flex-start',
                        maxWidth: '80%',
                        padding: '0.75rem 1rem',
                        borderRadius: msg.role === 'user' ? '16px 16px 4px 16px' : '16px 16px 16px 4px',
                        background: msg.role === 'user' ? 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)' : '#f1f3f4',
                        color: msg.role === 'user' ? 'white' : '#333',
                        fontSize: '0.9rem',
                        lineHeight: 1.4,
                        whiteSpace: 'pre-wrap'
                      }}>
                        {msg.content}
                      </div>
                    ))}
                    {isChatLoading && (
                      <div style={{
                        alignSelf: 'flex-start',
                        padding: '0.75rem 1rem',
                        borderRadius: '16px 16px 16px 4px',
                        background: '#f1f3f4',
                        color: '#666'
                      }}>
                        ⏳ Thinking...
                      </div>
                    )}
                  </>
                )}
              </div>

              {/* Input */}
              <div style={{
                padding: '1rem',
                borderTop: '1px solid #eee',
                display: 'flex',
                gap: '0.5rem'
              }}>
                <input
                  type="text"
                  value={chatInput}
                  onChange={(e) => setChatInput(e.target.value)}
                  onKeyPress={(e) => e.key === 'Enter' && handleSendChat()}
                  placeholder="Type a message..."
                  style={{
                    flex: 1,
                    padding: '0.75rem 1rem',
                    borderRadius: '24px',
                    border: '1px solid #ddd',
                    outline: 'none',
                    fontSize: '0.9rem'
                  }}
                />
                <button
                  onClick={handleSendChat}
                  disabled={isChatLoading || !chatInput.trim()}
                  style={{
                    padding: '0.75rem 1rem',
                    borderRadius: '24px',
                    background: chatInput.trim() ? 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)' : '#ddd',
                    color: 'white',
                    border: 'none',
                    cursor: chatInput.trim() ? 'pointer' : 'not-allowed',
                    fontWeight: 600
                  }}
                >
                  Send
                </button>
              </div>
            </div>
          )}
        </>
      )}

      {/* User Preferences Modal */}
      {showPreferencesModal && (
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
          zIndex: 1001
        }}>
          <div style={{
            background: 'white',
            borderRadius: '16px',
            padding: '2rem',
            width: '450px',
            maxHeight: '80vh',
            overflowY: 'auto'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
              <h2 style={{ margin: 0 }}>⚙️ Preferences</h2>
              <button onClick={() => setShowPreferencesModal(false)} style={{ background: 'none', border: 'none', fontSize: '1.5rem', cursor: 'pointer' }}>×</button>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: 600 }}>Country</label>
                <input
                  type="text"
                  value={preferencesForm.country || ''}
                  onChange={(e) => setPreferencesForm(prev => ({ ...prev, country: e.target.value }))}
                  placeholder="e.g., India"
                  style={{ width: '100%', padding: '0.75rem', borderRadius: '8px', border: '1px solid #ddd' }}
                />
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: 600 }}>State</label>
                <input
                  type="text"
                  value={preferencesForm.state || ''}
                  onChange={(e) => setPreferencesForm(prev => ({ ...prev, state: e.target.value }))}
                  placeholder="e.g., Karnataka"
                  style={{ width: '100%', padding: '0.75rem', borderRadius: '8px', border: '1px solid #ddd' }}
                />
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: 600 }}>City</label>
                <input
                  type="text"
                  value={preferencesForm.city || ''}
                  onChange={(e) => setPreferencesForm(prev => ({ ...prev, city: e.target.value }))}
                  placeholder="e.g., Bangalore"
                  style={{ width: '100%', padding: '0.75rem', borderRadius: '8px', border: '1px solid #ddd' }}
                />
              </div>

              <div>
                <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: 600 }}>Preferred Cuisines</label>
                <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
                  <input
                    type="text"
                    value={newCuisine}
                    onChange={(e) => setNewCuisine(e.target.value)}
                    onKeyPress={(e) => e.key === 'Enter' && handleAddCuisine()}
                    placeholder="e.g., South Indian"
                    style={{ flex: 1, padding: '0.75rem', borderRadius: '8px', border: '1px solid #ddd' }}
                  />
                  <button onClick={handleAddCuisine} className="btn btn-primary" style={{ padding: '0.75rem 1rem' }}>Add</button>
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                  {(preferencesForm.preferred_cuisines || []).map((cuisine, idx) => (
                    <span key={idx} style={{
                      background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                      color: 'white',
                      padding: '0.4rem 0.8rem',
                      borderRadius: '20px',
                      fontSize: '0.85rem',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '0.5rem'
                    }}>
                      {cuisine}
                      <button onClick={() => handleRemoveCuisine(cuisine)} style={{ background: 'none', border: 'none', color: 'white', cursor: 'pointer', fontSize: '1rem' }}>×</button>
                    </span>
                  ))}
                </div>
              </div>

              <div style={{ display: 'flex', gap: '1rem', marginTop: '1rem' }}>
                <button onClick={() => setShowPreferencesModal(false)} className="btn btn-secondary" style={{ flex: 1 }}>Cancel</button>
                <button onClick={handleSavePreferences} className="btn btn-primary" style={{ flex: 1 }}>Save Preferences</button>
              </div>
            </div>
          </div>
        </div>
      )}
      <CanIEatModal
        isOpen={showCanIEatModal}
        onClose={() => setShowCanIEatModal(false)}
        checkResult={canIEatResult}
        foodName={pendingMealLog?.name}
        onCheck={(query) => handleCheckFood({ query })}
        onReset={() => setCanIEatResult(null)}
        onLog={async () => {
          try {
            const food = canIEatResult.food;
            await logMeal({
              name: food.name,
              ingredients: pendingMealLog.ingredients || [`1 serving ${food.name}`],
              calories: food.calories,
              protein: food.protein,
              fat: food.fat,
              carbs: food.carbs,
              was_override: false
            });
            alert("Meal logged successfully!");
            setShowCanIEatModal(false);
            setCanIEatResult(null);
            loadData(); // Refresh remaining stats
          } catch (err) {
            console.error("Failed to log meal from modal", err);
            alert("Failed to log meal: " + err.message);
          }
        }}
        onLogSmall={() => {
          setShowCanIEatModal(false);
          setActiveTab('log-meal');
          setMealName(pendingMealLog.name + " (Small)");
          setMealIngredients(pendingMealLog.ingredients);
          alert("Please adjust quantity to be smaller.");
        }}
      />
    </div >
  );
}

export default App;
