import React, { createContext, useContext, useState, useEffect } from 'react';
import { apiClient as api, type LoginRequest, type AuthResponse } from '../lib/api';

interface User {
    id: number;
    username: string;
    role: string;
}

interface AuthContextType {
    user: User | null;
    isAuthenticated: boolean;
    login: (data: LoginRequest) => Promise<void>;
    logout: () => void;
    checkAuth: () => boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        // Hydrate from localStorage
        const token = localStorage.getItem('token');
        const savedUser = localStorage.getItem('user');

        if (token && savedUser) {
            try {
                setUser(JSON.parse(savedUser));
                setIsAuthenticated(true);
                api.setToken(token);
            } catch (e) {
                console.error("Failed to parse user from local storage", e);
                localStorage.removeItem('token');
                localStorage.removeItem('user');
            }
        }
        setLoading(false);
    }, []);

    const login = async (data: LoginRequest) => {
        const response = await api.login(data);
        setUser(response.user);
        setIsAuthenticated(true);
        localStorage.setItem('user', JSON.stringify(response.user));
    };

    const logout = () => {
        setUser(null);
        setIsAuthenticated(false);
        api.setToken(null);
        localStorage.removeItem('user');
        localStorage.removeItem('token');
    };

    const checkAuth = () => {
        return !!localStorage.getItem('token');
    };

    return (
        <AuthContext.Provider value={{ user, isAuthenticated, login, logout, checkAuth }}>
            {!loading && children}
        </AuthContext.Provider>
    );
}

export const useAuth = () => {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
};
