const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

const getAuthHeader = () => {
  const token = localStorage.getItem('token');
  // For the simplified backend, token is the user ID if not a real JWT
  // But we use 'Bearer <token>' as expected by middleware
  return token ? { 'Authorization': `Bearer ${token}` } : {};
};

export const fetchPantry = async () => {
  const res = await fetch(`${API_BASE}/pantry`, {
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to fetch pantry');
  return res.json();
};

export const updatePantryItem = async (itemID, manualQuantity) => {
  const res = await fetch(`${API_BASE}/pantry/${itemID}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify({ manual_quantity: manualQuantity })
  });
  if (!res.ok) throw new Error('Failed to update item');
};

export const deletePantryItem = async (itemID) => {
  const res = await fetch(`${API_BASE}/pantry/${itemID}`, {
    method: 'DELETE',
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to delete item');
};

export const bulkDeletePantryItems = async (itemIDs) => {
  const res = await fetch(`${API_BASE}/pantry/bulk-delete`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify({ item_ids: itemIDs })
  });
  if (!res.ok) throw new Error('Failed to bulk delete items');
};

export const fetchLowStock = async () => {
  const res = await fetch(`${API_BASE}/pantry/low-stock`, {
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to fetch low stock');
  return res.json();
};

export const extractItems = async (file) => {
  const formData = new FormData();
  formData.append('image', file);

  const response = await fetch(`${API_BASE}/items/extract`, {
    method: 'POST',
    headers: getAuthHeader(),
    body: formData,
  });

  if (!response.ok) throw new Error('Extraction failed');
  return response.json();
};

export const ingestOrder = async (orderData) => {
  // We need to inject the user_id from the token/state
  // For now, assume the backend extraction return include it or we add it here
  const response = await fetch(`${API_BASE}/ingest/order`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': 'secret-key',
    },
    body: JSON.stringify(orderData),
  });

  if (!response.ok) throw new Error('Ingestion failed');
  return response.json();
};

export const suggestMeal = async (inventory) => {
  const response = await fetch(`${API_BASE}/llm/suggest-meal`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ inventory }),
  });

  if (!response.ok) {
    const err = await response.json();
    throw new Error(err.error || 'Failed to get meal suggestions');
  }
  return response.json();
};

export const suggestMealPersonalized = async (inventory, goals, timeOfDay) => {
  const response = await fetch(`${API_BASE}/llm/suggest-meal-personalized`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify({ inventory, goals, time_of_day: timeOfDay }),
  });

  if (!response.ok) {
    const err = await response.json();
    throw new Error(err.error || 'Failed to get meal suggestions');
  }
  return response.json();
};

export const fetchGoals = async () => {
  const res = await fetch(`${API_BASE}/goals`, {
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to fetch goals');
  return res.json();
};

export const createGoal = async (title, description) => {
  const res = await fetch(`${API_BASE}/goals`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify({ title, description })
  });
  if (!res.ok) throw new Error('Failed to create goal');
  return res.json();
};

export const deleteGoal = async (goalId) => {
  const res = await fetch(`${API_BASE}/goals/${goalId}`, {
    method: 'DELETE',
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to delete goal');
};


export const logMeal = async (meal) => {
  const res = await fetch(`${API_BASE}/meals/log`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify({
      name: meal.name,
      ingredients: meal.ingredients,
      calories: meal.calories,
      protein: meal.protein,
      fat: meal.fat,
      carbs: meal.carbs,
      was_override: meal.was_override
    })
  });

  if (res.status === 403) {
    const data = await res.json();
    const err = new Error(data.error || 'Meal blocked');
    err.status = 403;
    err.reason = data.blocked_reason;
    throw err;
  }

  if (!res.ok) throw new Error('Failed to log meal');
  return res.json();
};


export const fetchMealHistory = async () => {
  const res = await fetch(`${API_BASE}/meals`, {
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to fetch meal history');
  return res.json();
};

export const deleteMealLog = async (mealId) => {
  const res = await fetch(`${API_BASE}/meals/${mealId}`, {
    method: 'DELETE',
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to delete meal');
  return res.json();
};

export const sendChatMessage = async (message, history, inventory, goals) => {
  const res = await fetch(`${API_BASE}/llm/chat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify({ message, history, inventory, goals })
  });
  if (!res.ok) throw new Error('Failed to get chat response');
  return res.json();
};

export const saveConversation = async (messages) => {
  const res = await fetch(`${API_BASE}/conversations`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify({ messages })
  });
  if (!res.ok) throw new Error('Failed to save conversation');
  return res.json();
};

export const fetchConversations = async () => {
  const res = await fetch(`${API_BASE}/conversations`, {
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to fetch conversations');
  return res.json();
};

export const fetchUserPreferences = async () => {
  const res = await fetch(`${API_BASE}/preferences`, {
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to fetch preferences');
  return res.json();
};

export const updateUserPreferences = async (preferences) => {
  const res = await fetch(`${API_BASE}/preferences`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify(preferences)
  });
  if (!res.ok) throw new Error('Failed to update preferences');
  return res.json();
};

export const fetchDishSamples = async (cuisine, region) => {
  const params = new URLSearchParams();
  if (cuisine) params.append('cuisine', cuisine);
  if (region) params.append('region', region);
  const res = await fetch(`${API_BASE}/dish-samples?${params}`, {
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to fetch dish samples');
  return res.json();
};

export const createDishSamplesBulk = async (dishes) => {
  const res = await fetch(`${API_BASE}/dish-samples/bulk`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify(dishes)
  });
  if (!res.ok) throw new Error('Failed to create dish samples');
  return res.json();
};

export const addPantryItem = async (name, quantity, unit) => {
  const res = await fetch(`${API_BASE}/pantry/add`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify({ name, quantity, unit })
  });
  if (!res.ok) throw new Error('Failed to add pantry item');
  return res.json();
};

// Can I Eat This?
export const checkFoodPermission = async (foodData) => {
  const res = await fetch(`${API_BASE}/can-i-eat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader(),
    },
    body: JSON.stringify(foodData),
  });
  if (!res.ok) throw new Error('Advice check failed');
  return res.json();
};

export const fetchRemainingDayState = async () => {
  const res = await fetch(`${API_BASE}/remaining-day-state`, {
    headers: getAuthHeader()
  });
  if (!res.ok) {
    if (res.status === 404) return null;
    const err = await res.json().catch(() => ({}));
    if (err.status === "no_targets") return null;
    throw new Error('Failed to fetch remaining day state');
  }
  const data = await res.json();
  if (data.status === "no_targets") return null;
  return data;
};

export const setGoalMacroTargets = async (goalId, targets) => {
  const res = await fetch(`${API_BASE}/goals/${goalId}/targets`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...getAuthHeader()
    },
    body: JSON.stringify(targets)
  });
  if (!res.ok) throw new Error('Failed to set targets');
  return res.json();
};

export const validateMeal = async (macros) => {
  const params = new URLSearchParams();
  if (macros.calories) params.append('calories', macros.calories);
  if (macros.protein) params.append('protein', macros.protein);

  const res = await fetch(`${API_BASE}/meals/validate?${params.toString()}`, {
    headers: getAuthHeader()
  });
  if (!res.ok) throw new Error('Failed to validate meal');
  return res.json();
};


