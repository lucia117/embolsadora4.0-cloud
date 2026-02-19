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

// TenantUpdateRequest define la estructura para actualizar un tenant (con campos opcionales)
type TenantUpdateRequest struct {
	Name        *string        `json:"name"`
	CompanyName *string        `json:"companyName"`
	Subdomain   *string        `json:"subdomain"`
	Description *string        `json:"description"`
	IsActive    *bool          `json:"isActive"`
	Theme       *ThemeUpdate   `json:"theme"`
	Address     *AddressUpdate `json:"address"`
}
