const API_BASE = 'http://localhost:8080';

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
  formData.append('invoice', file);

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
      protein: meal.protein
    })
  });
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
