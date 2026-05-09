--
-- PostgreSQL database dump
--



SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

-- *not* creating schema, since initdb creates it


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS '';


--
-- Name: update_alarm_rules_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_alarm_rules_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


--
-- Name: update_dashboard_layouts_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_dashboard_layouts_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;


--
-- Name: update_edge_devices_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_edge_devices_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;


--
-- Name: update_permissions_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_permissions_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


--
-- Name: update_updated_at_column(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


--
-- Name: update_users_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_users_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: alarm_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alarm_rules (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    metric text NOT NULL,
    operator text NOT NULL,
    threshold numeric(15,4) NOT NULL,
    severity text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT alarm_rules_operator_check CHECK ((operator = ANY (ARRAY['gt'::text, 'lt'::text, 'gte'::text, 'lte'::text, 'eq'::text]))),
    CONSTRAINT alarm_rules_severity_check CHECK ((severity = ANY (ARRAY['info'::text, 'warning'::text, 'critical'::text])))
);


--
-- Name: dashboard_layouts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dashboard_layouts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    widgets jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    user_id uuid NOT NULL
);


--
-- Name: device_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.device_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    device_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    check_type character varying(20) NOT NULL,
    checked_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    overall_status character varying(20) NOT NULL,
    summary text,
    details jsonb,
    user_id uuid NOT NULL,
    user_email character varying(254) NOT NULL,
    CONSTRAINT device_events_check_type_check CHECK (((check_type)::text = ANY ((ARRAY['STATUS'::character varying, 'HEALTH_CHECK'::character varying])::text[]))),
    CONSTRAINT device_events_overall_status_check CHECK (((overall_status)::text = ANY ((ARRAY['OK'::character varying, 'DEGRADED'::character varying, 'ERROR'::character varying, 'UNKNOWN'::character varying])::text[])))
);


--
-- Name: edge_devices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.edge_devices (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    machine_id character varying(100) NOT NULL,
    edge_type character varying(50) NOT NULL,
    raspberry_base_url text NOT NULL,
    plc_address character varying(255),
    status character varying(20) DEFAULT 'ACTIVE'::character varying NOT NULL,
    last_seen_at timestamp with time zone,
    last_health_check_at timestamp with time zone,
    last_health_status character varying(20) DEFAULT 'UNKNOWN'::character varying NOT NULL,
    last_health_summary text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT edge_devices_edge_type_check CHECK (((edge_type)::text = 'RASPBERRY_PLC'::text)),
    CONSTRAINT edge_devices_last_health_status_check CHECK (((last_health_status)::text = ANY ((ARRAY['OK'::character varying, 'DEGRADED'::character varying, 'ERROR'::character varying, 'UNKNOWN'::character varying])::text[]))),
    CONSTRAINT edge_devices_status_check CHECK (((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'DISABLED'::character varying])::text[])))
);


--
-- Name: log_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.log_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    severity character varying(20) NOT NULL,
    event_type character varying(50) NOT NULL,
    source_id uuid,
    machine_id uuid,
    message text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT log_entries_event_type_check CHECK (((event_type)::text = ANY ((ARRAY['alarm_triggered'::character varying, 'alarm_resolved'::character varying, 'device_connected'::character varying, 'device_disconnected'::character varying, 'device_state_changed'::character varying, 'user_action'::character varying, 'system'::character varying])::text[]))),
    CONSTRAINT log_entries_severity_check CHECK (((severity)::text = ANY ((ARRAY['info'::character varying, 'warning'::character varying, 'critical'::character varying, 'error'::character varying])::text[])))
);


--
-- Name: log_retention_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.log_retention_policies (
    tenant_id uuid NOT NULL,
    retention_days integer DEFAULT 90 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    next_purge_at timestamp with time zone DEFAULT (now() + '1 day'::interval) NOT NULL,
    CONSTRAINT log_retention_policies_retention_days_check CHECK ((retention_days > 0))
);


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    title text NOT NULL,
    message text NOT NULL,
    severity character varying(20) NOT NULL,
    status character varying(20) DEFAULT 'unread'::character varying NOT NULL,
    alarm_rule_id uuid,
    machine_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    acknowledged_at timestamp with time zone,
    closed_at timestamp with time zone,
    CONSTRAINT notifications_severity_check CHECK (((severity)::text = ANY ((ARRAY['info'::character varying, 'warning'::character varying, 'critical'::character varying, 'error'::character varying])::text[]))),
    CONSTRAINT notifications_status_check CHECK (((status)::text = ANY ((ARRAY['unread'::character varying, 'acknowledged'::character varying, 'closed'::character varying])::text[])))
);


--
-- Name: permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permissions (
    id text NOT NULL,
    name text NOT NULL,
    section text NOT NULL,
    description text NOT NULL,
    is_system_permission boolean DEFAULT false NOT NULL,
    tenant_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_custom_perm_has_tenant CHECK ((NOT ((is_system_permission = false) AND (tenant_id IS NULL)))),
    CONSTRAINT chk_system_perm_no_tenant CHECK ((NOT ((is_system_permission = true) AND (tenant_id IS NOT NULL)))),
    CONSTRAINT permissions_description_check CHECK ((char_length(description) > 0)),
    CONSTRAINT permissions_name_check CHECK ((char_length(name) >= 3)),
    CONSTRAINT permissions_section_check CHECK ((char_length(section) > 0))
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id character varying(50) NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    is_system_role boolean DEFAULT false NOT NULL,
    is_global boolean DEFAULT false NOT NULL,
    tenant_id uuid,
    permissions jsonb DEFAULT '[]'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: tenants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenants (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    company_name character varying(255) NOT NULL,
    subdomain character varying(100) NOT NULL,
    description text,
    is_active boolean DEFAULT true,
    primary_color character varying(7) DEFAULT '#3b82f6'::character varying,
    secondary_color character varying(7) DEFAULT '#6366f1'::character varying,
    accent_color character varying(7) DEFAULT '#8b5cf6'::character varying,
    text_color character varying(7) DEFAULT '#1f2937'::character varying,
    background_color character varying(7) DEFAULT '#ffffff'::character varying,
    logo_url character varying(500),
    favicon_url character varying(500),
    street character varying(255),
    city character varying(100),
    state character varying(100),
    postal_code character varying(20),
    country character varying(100),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_invitations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_invitations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    email text NOT NULL,
    role_id character varying(50) NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    invited_by uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone DEFAULT (now() + '7 days'::interval) NOT NULL,
    CONSTRAINT user_invitations_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'accepted'::text, 'revoked'::text, 'expired'::text])))
);


--
-- Name: user_tenant_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_tenant_roles (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    role_id character varying(50),
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    assigned_by uuid,
    assigned_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_tenant_roles_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'pending'::character varying, 'revoked'::character varying, 'suspended'::character varying])::text[])))
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid NOT NULL,
    email character varying(255) NOT NULL,
    name character varying(255),
    image text,
    tenant_id uuid,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    supabase_user_id text,
    auth_provider text,
    email_verified_at timestamp with time zone,
    last_login_at timestamp with time zone,
    password_change_required boolean DEFAULT false NOT NULL,
    first_name character varying(100),
    last_name character varying(100),
    role character varying(50) DEFAULT 'user'::character varying NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: alarm_rules alarm_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_rules
    ADD CONSTRAINT alarm_rules_pkey PRIMARY KEY (id);


--
-- Name: dashboard_layouts dashboard_layouts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dashboard_layouts
    ADD CONSTRAINT dashboard_layouts_pkey PRIMARY KEY (id);


--
-- Name: device_events device_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_events
    ADD CONSTRAINT device_events_pkey PRIMARY KEY (id);


--
-- Name: edge_devices edge_devices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edge_devices
    ADD CONSTRAINT edge_devices_pkey PRIMARY KEY (id);


--
-- Name: log_entries log_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.log_entries
    ADD CONSTRAINT log_entries_pkey PRIMARY KEY (id);


--
-- Name: log_retention_policies log_retention_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.log_retention_policies
    ADD CONSTRAINT log_retention_policies_pkey PRIMARY KEY (tenant_id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: permissions permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_pkey PRIMARY KEY (id);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_subdomain_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_subdomain_key UNIQUE (subdomain);


--
-- Name: edge_devices uq_edge_devices_tenant_machine; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edge_devices
    ADD CONSTRAINT uq_edge_devices_tenant_machine UNIQUE (tenant_id, machine_id);


--
-- Name: user_invitations user_invitations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_invitations
    ADD CONSTRAINT user_invitations_pkey PRIMARY KEY (id);


--
-- Name: user_tenant_roles user_tenant_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_tenant_roles
    ADD CONSTRAINT user_tenant_roles_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_supabase_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_supabase_user_id_key UNIQUE (supabase_user_id);


--
-- Name: users users_tenant_id_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_tenant_id_email_key UNIQUE (tenant_id, email);


--
-- Name: idx_alarm_rules_tenant_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alarm_rules_tenant_enabled ON public.alarm_rules USING btree (tenant_id, enabled);


--
-- Name: idx_alarm_rules_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_alarm_rules_tenant_id ON public.alarm_rules USING btree (tenant_id);


--
-- Name: idx_dashboard_layouts_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dashboard_layouts_tenant_id ON public.dashboard_layouts USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_dashboard_layouts_tenant_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dashboard_layouts_tenant_user ON public.dashboard_layouts USING btree (tenant_id, user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_dashboard_layouts_tenant_user_name_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dashboard_layouts_tenant_user_name_active ON public.dashboard_layouts USING btree (tenant_id, user_id, name) WHERE (deleted_at IS NULL);


--
-- Name: idx_device_events_device_checked_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_events_device_checked_at ON public.device_events USING btree (device_id, checked_at DESC);


--
-- Name: idx_device_events_device_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_events_device_id ON public.device_events USING btree (device_id);


--
-- Name: idx_device_events_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_device_events_tenant_id ON public.device_events USING btree (tenant_id);


--
-- Name: idx_edge_devices_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edge_devices_tenant_id ON public.edge_devices USING btree (tenant_id);


--
-- Name: idx_edge_devices_tenant_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edge_devices_tenant_status ON public.edge_devices USING btree (tenant_id, status);


--
-- Name: idx_log_entries_event_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_log_entries_event_type ON public.log_entries USING btree (tenant_id, event_type, created_at DESC);


--
-- Name: idx_log_entries_machine; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_log_entries_machine ON public.log_entries USING btree (tenant_id, machine_id) WHERE (machine_id IS NOT NULL);


--
-- Name: idx_log_entries_message_fts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_log_entries_message_fts ON public.log_entries USING gin (to_tsvector('spanish'::regconfig, message));


--
-- Name: idx_log_entries_severity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_log_entries_severity ON public.log_entries USING btree (tenant_id, severity, created_at DESC);


--
-- Name: idx_log_entries_tenant_cursor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_log_entries_tenant_cursor ON public.log_entries USING btree (tenant_id, created_at DESC, id DESC);


--
-- Name: idx_notifications_tenant_list; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_tenant_list ON public.notifications USING btree (tenant_id, created_at DESC);


--
-- Name: idx_notifications_tenant_severity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_tenant_severity ON public.notifications USING btree (tenant_id, severity, created_at DESC);


--
-- Name: idx_notifications_tenant_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_tenant_status ON public.notifications USING btree (tenant_id, status, created_at DESC);


--
-- Name: idx_permissions_system; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_permissions_system ON public.permissions USING btree (is_system_permission) WHERE (is_system_permission = true);


--
-- Name: idx_permissions_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_permissions_tenant_id ON public.permissions USING btree (tenant_id) WHERE (tenant_id IS NOT NULL);


--
-- Name: idx_roles_tenant_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_roles_tenant_active ON public.roles USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_roles_tenant_name_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_roles_tenant_name_active ON public.roles USING btree (tenant_id, name) WHERE ((deleted_at IS NULL) AND (is_system_role = false));


--
-- Name: idx_tenants_is_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tenants_is_active ON public.tenants USING btree (is_active);


--
-- Name: idx_tenants_subdomain; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tenants_subdomain ON public.tenants USING btree (subdomain);


--
-- Name: idx_user_invitations_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_invitations_email ON public.user_invitations USING btree (email);


--
-- Name: idx_user_invitations_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_invitations_pending ON public.user_invitations USING btree (tenant_id, email) WHERE (status = 'pending'::text);


--
-- Name: idx_user_invitations_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_invitations_tenant ON public.user_invitations USING btree (tenant_id);


--
-- Name: idx_users_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_deleted_at ON public.users USING btree (deleted_at);


--
-- Name: idx_users_supabase_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_supabase_user_id ON public.users USING btree (supabase_user_id);


--
-- Name: idx_users_tenant_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_tenant_deleted ON public.users USING btree (tenant_id, deleted_at);


--
-- Name: idx_users_tenant_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_tenant_email ON public.users USING btree (tenant_id, email);


--
-- Name: idx_users_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_tenant_id ON public.users USING btree (tenant_id);


--
-- Name: idx_utr_active_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_utr_active_unique ON public.user_tenant_roles USING btree (user_id, tenant_id) WHERE ((status)::text = 'active'::text);


--
-- Name: idx_utr_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_utr_status ON public.user_tenant_roles USING btree (status);


--
-- Name: idx_utr_tenant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_utr_tenant_id ON public.user_tenant_roles USING btree (tenant_id);


--
-- Name: idx_utr_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_utr_user_id ON public.user_tenant_roles USING btree (user_id);


--
-- Name: alarm_rules trg_alarm_rules_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_alarm_rules_updated_at BEFORE UPDATE ON public.alarm_rules FOR EACH ROW EXECUTE FUNCTION public.update_alarm_rules_updated_at();


--
-- Name: dashboard_layouts trg_dashboard_layouts_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_dashboard_layouts_updated_at BEFORE UPDATE ON public.dashboard_layouts FOR EACH ROW EXECUTE FUNCTION public.update_dashboard_layouts_updated_at();


--
-- Name: edge_devices trg_edge_devices_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_edge_devices_updated_at BEFORE UPDATE ON public.edge_devices FOR EACH ROW EXECUTE FUNCTION public.update_edge_devices_updated_at();


--
-- Name: permissions trg_permissions_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_permissions_updated_at BEFORE UPDATE ON public.permissions FOR EACH ROW EXECUTE FUNCTION public.update_permissions_updated_at();


--
-- Name: users trigger_update_users_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_update_users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.update_users_updated_at();


--
-- Name: log_retention_policies update_log_retention_policies_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_log_retention_policies_updated_at BEFORE UPDATE ON public.log_retention_policies FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: alarm_rules alarm_rules_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alarm_rules
    ADD CONSTRAINT alarm_rules_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dashboard_layouts dashboard_layouts_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dashboard_layouts
    ADD CONSTRAINT dashboard_layouts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dashboard_layouts dashboard_layouts_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dashboard_layouts
    ADD CONSTRAINT dashboard_layouts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: device_events device_events_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.device_events
    ADD CONSTRAINT device_events_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.edge_devices(id) ON DELETE CASCADE;


--
-- Name: edge_devices edge_devices_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edge_devices
    ADD CONSTRAINT edge_devices_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: permissions permissions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: roles roles_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: user_invitations user_invitations_invited_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_invitations
    ADD CONSTRAINT user_invitations_invited_by_fkey FOREIGN KEY (invited_by) REFERENCES public.users(id);


--
-- Name: user_invitations user_invitations_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_invitations
    ADD CONSTRAINT user_invitations_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id);


--
-- Name: user_invitations user_invitations_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_invitations
    ADD CONSTRAINT user_invitations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: user_tenant_roles user_tenant_roles_assigned_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_tenant_roles
    ADD CONSTRAINT user_tenant_roles_assigned_by_fkey FOREIGN KEY (assigned_by) REFERENCES public.users(id);


--
-- Name: user_tenant_roles user_tenant_roles_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_tenant_roles
    ADD CONSTRAINT user_tenant_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id);


--
-- Name: user_tenant_roles user_tenant_roles_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_tenant_roles
    ADD CONSTRAINT user_tenant_roles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: user_tenant_roles user_tenant_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_tenant_roles
    ADD CONSTRAINT user_tenant_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: users users_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


