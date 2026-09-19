import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import client from '../api/client';
import { Event } from '../api/types';

const fetchEvents = async (): Promise<Event[]> => {
  const res = await client.get('/events');
  return res.data;
};

const Events: React.FC = () => {
  const [severityFilter, setSeverityFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string>('');

  const { data: events, isLoading, error } = useQuery({
    queryKey: ['events'],
    queryFn: fetchEvents,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading events...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load events
      </div>
    );
  }

  const filteredEvents = events?.filter((event) => {
    if (severityFilter && event.severity !== severityFilter) return false;
    if (typeFilter && event.type !== typeFilter) return false;
    return true;
  });

  const uniqueSeverities = [...new Set(events?.map((e) => e.severity) ?? [])];
  const uniqueTypes = [...new Set(events?.map((e) => e.type) ?? [])];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Events & Audit Log</h1>

      <div className="bg-slate-800 p-4 rounded-lg border border-slate-700 flex flex-wrap gap-4">
        <div>
          <label className="block text-slate-300 text-sm font-medium mb-2">Severity</label>
          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value)}
            className="px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
          >
            <option value="">All</option>
            {uniqueSeverities.map((sev) => (
              <option key={sev} value={sev}>{sev}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-slate-300 text-sm font-medium mb-2">Type</label>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
          >
            <option value="">All</option>
            {uniqueTypes.map((type) => (
              <option key={type} value={type}>{type}</option>
            ))}
          </select>
        </div>
        {(severityFilter || typeFilter) && (
          <button
            onClick={() => {
              setSeverityFilter('');
              setTypeFilter('');
            }}
            className="self-end text-sm text-blue-400 hover:text-blue-300"
          >
            Clear filters
          </button>
        )}
      </div>

      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-700/50">
            <tr>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Severity</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Type</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Message</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Timestamp</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {filteredEvents?.map((event) => (
              <tr key={event.id} className="hover:bg-slate-700/30">
                <td className="px-4 py-3">
                  <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                    event.severity === 'error' ? 'bg-red-900/50 text-red-300' :
                    event.severity === 'warning' ? 'bg-yellow-900/50 text-yellow-300' :
                    event.severity === 'info' ? 'bg-blue-900/50 text-blue-300' :
                    'bg-slate-700 text-slate-300'
                  }`}>
                    {event.severity}
                  </span>
                </td>
                <td className="px-4 py-3 text-slate-300 text-sm">{event.type}</td>
                <td className="px-4 py-3 text-slate-100">{event.message}</td>
                <td className="px-4 py-3 text-slate-400 text-sm">
                  {new Date(event.created_at).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {(!filteredEvents || filteredEvents.length === 0) && (
          <div className="text-center py-8 text-slate-400">No events found</div>
        )}
      </div>
    </div>
  );
};

export default Events;
