package models

// ThemeUpdate represents the theme configuration for tenant update
type ThemeUpdate struct {
	PrimaryColor    *string `json:"primaryColor"`
	SecondaryColor  *string `json:"secondaryColor"`
	AccentColor     *string `json:"accentColor"`
	TextColor       *string `json:"textColor"`
	BackgroundColor *string `json:"backgroundColor"`
	LogoUrl         *string `json:"logoUrl"`
	FaviconUrl      *string `json:"faviconUrl"`
}

// AddressUpdate represents the address information for tenant update
type AddressUpdate struct {
	Street     *string `json:"street"`
	City       *string `json:"city"`
	State      *string `json:"state"`
	PostalCode *string `json:"postalCode"`
	Country    *string `json:"country"`
}

// SettingsUpdate represents the localization/preferences sub-object for tenant update
type SettingsUpdate struct {
	Locale     *string `json:"locale"`
	Timezone   *string `json:"timezone"`
	DateFormat *string `json:"dateFormat"`
	TimeFormat *string `json:"timeFormat"`
	Currency   *string `json:"currency"`
}

// TenantUpdateRequest define la estructura para actualizar un tenant (con campos opcionales)
type TenantUpdateRequest struct {
	Name           *string         `json:"name"`
	CompanyName    *string         `json:"companyName"`
	Subdomain      *string         `json:"subdomain"`
	Description    *string         `json:"description"`
	IsActive       *bool           `json:"isActive"`
	ContactEmail   *string         `json:"contactEmail"`
	CompanyWebsite *string         `json:"companyWebsite"`
	Theme          *ThemeUpdate    `json:"theme"`
	Address        *AddressUpdate  `json:"address"`
	Settings       *SettingsUpdate `json:"settings"`
}
